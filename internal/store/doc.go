package store

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"mufish/internal/model"
)

// OpUser 操作人信息。
type OpUser struct {
	ID   int64
	Name string
}

// stockMoveReq 库存变动请求。
type stockMoveReq struct {
	WarehouseID int64
	ProductID   int64
	Qty         model.Qty   // 正数入库，负数出库
	Price       model.Price // 入库单价；出库忽略，按移动加权成本计算
	BizType     string
	BizID       int64
	BizNo       string
	Remark      string
	User        OpUser
	// Strict 为 true 时库存不足则报错。
	Strict bool
}

// stockMoveRes 库存变动结果。
type stockMoveRes struct {
	BalanceAfter model.Qty   // 变动后结存数量
	CostPrice    model.Price // 本次出 / 入库使用的成本单价
	Amount       model.Money // 本次入 / 出库金额（绝对值）
}

// applyStock 在事务内更新库存余额并写入库存流水。
// 成本核算采用移动加权平均法：
//   - 入库：结存金额增加，结存单价 = 结存金额 / 结存数量
//   - 出库：出库金额 = 结存单价 × 出库数量，结存单价不变
func applyStock(ctx context.Context, tx *sql.Tx, req stockMoveReq) (stockMoveRes, error) {
	var res stockMoveRes
	if req.Qty == 0 {
		return res, fmt.Errorf("%w: 数量不能为 0", ErrBadRequest)
	}

	var curQty float64
	var curAmount, curPrice int64
	err := tx.QueryRowContext(ctx,
		`SELECT qty, amount, cost_price FROM stocks WHERE warehouse_id=? AND product_id=?`,
		req.WarehouseID, req.ProductID).Scan(&curQty, &curAmount, &curPrice)
	if err != nil && err != sql.ErrNoRows {
		return res, err
	}

	delta := req.Qty.Float()
	newQty := round4(curQty + delta)

	var movePrice, moveAmount, newAmount, newPrice int64

	if delta > 0 {
		movePrice = int64(req.Price)
		moveAmount = int64(model.AmountOf(req.Qty, req.Price))
		if moveAmount < 0 {
			moveAmount = -moveAmount
		}
		newAmount = curAmount + moveAmount
		if newQty > 0 {
			newPrice = int64(math.Round(float64(newAmount) * 100 / newQty))
		} else {
			newPrice = curPrice
		}
	} else {
		movePrice = curPrice
		outQty := -delta
		moveAmount = int64(math.Round(float64(curPrice) * outQty / 100))
		if moveAmount > curAmount && curQty > 0 {
			// 成本不足时按现有结存金额冲减，避免出现负库存金额
			moveAmount = curAmount
		}
		newAmount = curAmount - moveAmount
		if newAmount < 0 {
			newAmount = 0
		}
		if newQty <= 0 {
			newQty = round4(newQty)
			if newQty == 0 {
				newAmount = 0
			}
		}
		newPrice = curPrice
	}

	if req.Strict && newQty < 0 {
		return res, fmt.Errorf("%w: 商品库存不足（当前结存 %s，本次需出库 %s）",
			ErrBadRequest, model.Qty(curQty).String(), model.Qty(-delta).String())
	}

	now := model.Now()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO stocks(warehouse_id, product_id, qty, cost_price, amount, updated_at)
		 VALUES(?,?,?,?,?,?)
		 ON CONFLICT(warehouse_id, product_id) DO UPDATE SET
		   qty=excluded.qty, cost_price=excluded.cost_price, amount=excluded.amount, updated_at=excluded.updated_at`,
		req.WarehouseID, req.ProductID, newQty, newPrice, newAmount, now); err != nil {
		return res, err
	}

	direction := 1
	if delta < 0 {
		direction = -1
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO stock_moves(warehouse_id, product_id, biz_type, biz_id, biz_no, direction,
		   qty, price, amount, balance_after, remark, created_by, created_by_name, created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		req.WarehouseID, req.ProductID, req.BizType, req.BizID, req.BizNo, direction,
		delta, movePrice, moveAmount, newQty, req.Remark, req.User.ID, req.User.Name, now); err != nil {
		return res, err
	}

	res.BalanceAfter = model.Qty(newQty)
	res.CostPrice = model.Price(movePrice)
	res.Amount = model.Money(moveAmount)
	return res, nil
}

// reverseMovesByBiz 冲销某单据产生的库存流水（按相反方向）。
func reverseMovesByBiz(ctx context.Context, tx *sql.Tx, bizType string, bizID int64, user OpUser) error {
	moves, err := query[model.StockMove](ctx, tx,
		`SELECT id, warehouse_id, product_id, biz_type, biz_id, biz_no, direction,
		   qty, price, amount, balance_after, remark, created_by, created_at
		 FROM stock_moves WHERE biz_type=? AND biz_id=? ORDER BY id DESC`, bizType, bizID)
	if err != nil {
		return err
	}
	for _, m := range moves {
		if m.Qty == 0 {
			continue
		}
		_, err := applyStock(ctx, tx, stockMoveReq{
			WarehouseID: m.WarehouseID,
			ProductID:   m.ProductID,
			Qty:         model.Qty(-m.Qty),
			Price:       m.Price,
			BizType:     m.BizType,
			BizID:       m.BizID,
			BizNo:       m.BizNo,
			Remark:      "作废冲销",
			User:        user,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

// ---------------------------------------------------------------------------
// 单据明细通用
// ---------------------------------------------------------------------------

// OrderItemInput 单据明细入参。
type OrderItemInput struct {
	ProductID int64       `json:"product_id"`
	Qty       model.Qty   `json:"qty"`
	Price     model.Price `json:"price"`
	Remark    string      `json:"remark"`
}

type productBrief struct {
	ID   int64  `db:"id"`
	Code string `db:"code"`
	Name string `db:"name"`
	Spec string `db:"spec"`
	Unit string `db:"unit"`
}

func loadProductBrief(ctx context.Context, tx *sql.Tx, id int64) (productBrief, error) {
	b, ok, err := queryRow[productBrief](ctx, tx,
		`SELECT id, code, name, spec, unit FROM products WHERE id=?`, id)
	if err != nil {
		return b, err
	}
	if !ok {
		return b, fmt.Errorf("%w: 商品 #%d 不存在", ErrBadRequest, id)
	}
	return b, nil
}

// docTotals 单据合计。
type docTotals struct {
	TotalQty       model.Qty
	TotalAmount    model.Money
	SettleAmount   model.Money
	DiscountAmount model.Money
	OtherFee       model.Money
}

func calcTotals(items []OrderItemInput, discount, otherFee model.Money) docTotals {
	var t docTotals
	for _, it := range items {
		t.TotalQty += it.Qty
		t.TotalAmount += model.AmountOf(it.Qty, it.Price)
	}
	t.DiscountAmount = discount
	t.OtherFee = otherFee
	t.SettleAmount = t.TotalAmount - discount + otherFee
	if t.SettleAmount < 0 {
		t.SettleAmount = 0
	}
	return t
}

// ---------------------------------------------------------------------------
// 采购入库单
// ---------------------------------------------------------------------------

// PurchaseQuery 采购单查询条件。
type PurchaseQuery struct {
	Keyword    string
	SupplierID int64
	Status     int
	From       string
	To         string
	Page       int
	Size       int
}

const purchaseCols = `o.id, o.order_no, o.supplier_id, o.supplier_name, o.warehouse_id, o.warehouse_name,
	o.order_date, o.status, o.total_qty, o.total_amount, o.discount_amount, o.other_fee,
	o.settle_amount, o.paid_amount, o.remark, o.created_by, o.created_by_name,
	o.confirmed_at, o.confirmed_by, o.created_at, o.updated_at`

// ListPurchases 分页查询采购单。
func (s *Store) ListPurchases(ctx context.Context, q PurchaseQuery) (model.Page[model.PurchaseOrder], error) {
	var out model.Page[model.PurchaseOrder]
	where, args := docFilter(q.Keyword, q.From, q.To, q.Status)
	if q.SupplierID > 0 {
		where = append(where, "o.supplier_id=?")
		args = append(args, q.SupplierID)
	}
	cond := strings.Join(where, " AND ")
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM purchase_orders o WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.PurchaseOrder](ctx, s.db,
		`SELECT `+purchaseCols+` FROM purchase_orders o WHERE `+cond+
			` ORDER BY o.order_date DESC, o.id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.PurchaseOrder]{List: list, Total: total, Page: page, Size: size}, nil
}

// GetPurchase 获取采购单（含明细）。
func (s *Store) GetPurchase(ctx context.Context, id int64) (model.PurchaseOrder, error) {
	o, ok, err := queryRow[model.PurchaseOrder](ctx, s.db,
		`SELECT `+purchaseCols+` FROM purchase_orders o WHERE o.id=?`, id)
	if err != nil {
		return o, err
	}
	if !ok {
		return o, ErrNotFound
	}
	items, err := query[model.OrderItem](ctx, s.db,
		`SELECT id, order_id AS doc_id, product_id, product_code, product_name, spec, unit,
		   qty, price, amount, remark, 0 AS sort
		 FROM purchase_items WHERE order_id=? ORDER BY id`, id)
	if err != nil {
		return o, err
	}
	o.Items = items
	return o, nil
}

// PurchaseInput 采购单入参。
type PurchaseInput struct {
	SupplierID     int64            `json:"supplier_id"`
	WarehouseID    int64            `json:"warehouse_id"`
	OrderDate      string           `json:"order_date"`
	DiscountAmount model.Money      `json:"discount_amount"`
	OtherFee       model.Money      `json:"other_fee"`
	Remark         string           `json:"remark"`
	Items          []OrderItemInput `json:"items"`
}

// SavePurchase 新建或修改采购单（仅草稿可修改）。
func (s *Store) SavePurchase(ctx context.Context, id int64, in PurchaseInput, user OpUser) (int64, error) {
	if len(in.Items) == 0 {
		return 0, fmt.Errorf("%w: 请至少录入一行商品明细", ErrBadRequest)
	}
	for i, it := range in.Items {
		if it.ProductID == 0 {
			return 0, fmt.Errorf("%w: 第 %d 行未选择商品", ErrBadRequest, i+1)
		}
		if it.Qty <= 0 {
			return 0, fmt.Errorf("%w: 第 %d 行数量必须大于 0", ErrBadRequest, i+1)
		}
	}
	in.OrderDate = model.NormDate(in.OrderDate)
	now := model.Now()

	err := s.Tx(ctx, func(tx *sql.Tx) error {
		supplier, err := loadPartnerName(ctx, tx, in.SupplierID)
		if err != nil {
			return err
		}
		wh, err := loadWarehouseName(ctx, tx, in.WarehouseID)
		if err != nil {
			return err
		}
		t := calcTotals(in.Items, in.DiscountAmount, in.OtherFee)

		if id > 0 {
			var status int
			if err := tx.QueryRowContext(ctx, `SELECT status FROM purchase_orders WHERE id=?`, id).Scan(&status); err != nil {
				if err == sql.ErrNoRows {
					return ErrNotFound
				}
				return err
			}
			if status != model.DocDraft {
				return fmt.Errorf("%w: 只有草稿状态的单据才能修改", ErrBadRequest)
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE purchase_orders SET supplier_id=?, supplier_name=?, warehouse_id=?, warehouse_name=?,
				   order_date=?, total_qty=?, total_amount=?, discount_amount=?, other_fee=?, settle_amount=?,
				   remark=?, updated_at=? WHERE id=?`,
				in.SupplierID, supplier, in.WarehouseID, wh, in.OrderDate,
				float64(t.TotalQty), t.TotalAmount, t.DiscountAmount, t.OtherFee, t.SettleAmount,
				in.Remark, now, id); err != nil {
				return wrapErr(err, "update purchase")
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM purchase_items WHERE order_id=?`, id); err != nil {
				return err
			}
			return insertPurchaseItems(ctx, tx, id, in.Items)
		}

		no, err := nextDocNo(ctx, tx, "purchase", "CG", in.OrderDate)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO purchase_orders(order_no, supplier_id, supplier_name, warehouse_id, warehouse_name,
			   order_date, status, total_qty, total_amount, discount_amount, other_fee, settle_amount,
			   paid_amount, remark, created_by, created_by_name, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,0,?,?,?,?,?,0,?,?,?,?,?)`,
			no, in.SupplierID, supplier, in.WarehouseID, wh, in.OrderDate,
			float64(t.TotalQty), t.TotalAmount, t.DiscountAmount, t.OtherFee, t.SettleAmount,
			in.Remark, user.ID, user.Name, now, now)
		if err != nil {
			return wrapErr(err, "insert purchase")
		}
		id, _ = res.LastInsertId()
		return insertPurchaseItems(ctx, tx, id, in.Items)
	})
	return id, err
}

func insertPurchaseItems(ctx context.Context, tx *sql.Tx, orderID int64, items []OrderItemInput) error {
	for i, it := range items {
		b, err := loadProductBrief(ctx, tx, it.ProductID)
		if err != nil {
			return err
		}
		amount := model.AmountOf(it.Qty, it.Price)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO purchase_items(order_id, product_id, product_code, product_name, spec, unit,
			   qty, price, amount, remark, sort) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			orderID, it.ProductID, b.Code, b.Name, b.Spec, b.Unit,
			float64(it.Qty), it.Price, amount, it.Remark, i); err != nil {
			return err
		}
	}
	return nil
}

// ConfirmPurchase 采购入库确认：增加库存。
func (s *Store) ConfirmPurchase(ctx context.Context, id int64, user OpUser) error {
	strict := !s.SettingBool(ctx, "allow_negative_stock", false)
	followPrice := s.SettingBool(ctx, "purchase_price_follow", true)
	_ = strict
	return s.Tx(ctx, func(tx *sql.Tx) error {
		o, ok, err := queryRow[model.PurchaseOrder](ctx, tx,
			`SELECT `+purchaseCols+` FROM purchase_orders o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if o.Status == model.DocConfirmed {
			return fmt.Errorf("%w: 该采购单已入库", ErrBadRequest)
		}
		if o.Status == model.DocVoided {
			return fmt.Errorf("%w: 该采购单已作废", ErrBadRequest)
		}
		items, err := query[model.OrderItem](ctx, tx,
			`SELECT id, order_id AS doc_id, product_id, product_code, product_name, spec, unit,
			   qty, price, amount, remark, 0 AS sort FROM purchase_items WHERE order_id=? ORDER BY id`, id)
		if err != nil {
			return err
		}
		for _, it := range items {
			if _, err := applyStock(ctx, tx, stockMoveReq{
				WarehouseID: o.WarehouseID,
				ProductID:   it.ProductID,
				Qty:         it.Qty,
				Price:       it.Price,
				BizType:     "purchase",
				BizID:       o.ID,
				BizNo:       o.OrderNo,
				User:        user,
			}); err != nil {
				return err
			}
			if followPrice && it.Price > 0 {
				if err := updateProductRefPrice(ctx, tx, it.ProductID, it.Price, true); err != nil {
					return err
				}
			}
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE purchase_orders SET status=1, confirmed_at=?, confirmed_by=?, updated_at=? WHERE id=?`,
			model.Now(), user.ID, model.Now(), id)
		return err
	})
}

// VoidPurchase 作废采购单（已入库的先冲减库存）。
func (s *Store) VoidPurchase(ctx context.Context, id int64, user OpUser) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		o, ok, err := queryRow[model.PurchaseOrder](ctx, tx,
			`SELECT `+purchaseCols+` FROM purchase_orders o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if o.Status == model.DocVoided {
			return fmt.Errorf("%w: 该采购单已作废", ErrBadRequest)
		}
		if o.PaidAmount > 0 {
			return fmt.Errorf("%w: 该采购单已有付款记录，请先作废相关付款单", ErrBadRequest)
		}
		if o.Status == model.DocConfirmed {
			if err := reverseMovesByBiz(ctx, tx, "purchase", id, user); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE purchase_orders SET status=2, updated_at=? WHERE id=?`, model.Now(), id)
		return err
	})
}

// DeletePurchase 删除采购单（仅草稿）。
func (s *Store) DeletePurchase(ctx context.Context, id int64) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		var status int
		if err := tx.QueryRowContext(ctx, `SELECT status FROM purchase_orders WHERE id=?`, id).Scan(&status); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		if status != model.DocDraft {
			return fmt.Errorf("%w: 只有草稿状态的单据才能删除，已入库请先作废", ErrBadRequest)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM purchase_items WHERE order_id=?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM purchase_orders WHERE id=?`, id)
		return err
	})
}

// ---------------------------------------------------------------------------
// 销售出库单
// ---------------------------------------------------------------------------

// SalesQuery 销售单查询条件。
type SalesQuery struct {
	Keyword    string
	CustomerID int64
	Status     int
	From       string
	To         string
	Page       int
	Size       int
}

const salesCols = `o.id, o.order_no, o.customer_id, o.customer_name, o.warehouse_id, o.warehouse_name,
	o.order_date, o.status, o.total_qty, o.total_amount, o.cost_amount, o.discount_amount, o.other_fee,
	o.settle_amount, o.received_amount, o.remark, o.created_by, o.created_by_name,
	o.confirmed_at, o.confirmed_by, o.created_at, o.updated_at`

// ListSales 分页查询销售单。
func (s *Store) ListSales(ctx context.Context, q SalesQuery) (model.Page[model.SalesOrder], error) {
	var out model.Page[model.SalesOrder]
	where, args := docFilter(q.Keyword, q.From, q.To, q.Status)
	if q.CustomerID > 0 {
		where = append(where, "o.customer_id=?")
		args = append(args, q.CustomerID)
	}
	cond := strings.Join(where, " AND ")
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM sales_orders o WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.SalesOrder](ctx, s.db,
		`SELECT `+salesCols+` FROM sales_orders o WHERE `+cond+
			` ORDER BY o.order_date DESC, o.id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.SalesOrder]{List: list, Total: total, Page: page, Size: size}, nil
}

// GetSales 获取销售单（含明细）。
func (s *Store) GetSales(ctx context.Context, id int64) (model.SalesOrder, error) {
	o, ok, err := queryRow[model.SalesOrder](ctx, s.db,
		`SELECT `+salesCols+` FROM sales_orders o WHERE o.id=?`, id)
	if err != nil {
		return o, err
	}
	if !ok {
		return o, ErrNotFound
	}
	items, err := query[model.OrderItem](ctx, s.db,
		`SELECT id, order_id AS doc_id, product_id, product_code, product_name, spec, unit,
		   qty, price, amount, remark, 0 AS sort FROM sales_items WHERE order_id=? ORDER BY id`, id)
	if err != nil {
		return o, err
	}
	o.Items = items
	return o, nil
}

// SalesInput 销售单入参。
type SalesInput struct {
	CustomerID     int64            `json:"customer_id"`
	WarehouseID    int64            `json:"warehouse_id"`
	OrderDate      string           `json:"order_date"`
	DiscountAmount model.Money      `json:"discount_amount"`
	OtherFee       model.Money      `json:"other_fee"`
	Remark         string           `json:"remark"`
	Items          []OrderItemInput `json:"items"`
}

// SaveSales 新建或修改销售单。
func (s *Store) SaveSales(ctx context.Context, id int64, in SalesInput, user OpUser) (int64, error) {
	if len(in.Items) == 0 {
		return 0, fmt.Errorf("%w: 请至少录入一行商品明细", ErrBadRequest)
	}
	for i, it := range in.Items {
		if it.ProductID == 0 {
			return 0, fmt.Errorf("%w: 第 %d 行未选择商品", ErrBadRequest, i+1)
		}
		if it.Qty <= 0 {
			return 0, fmt.Errorf("%w: 第 %d 行数量必须大于 0", ErrBadRequest, i+1)
		}
	}
	in.OrderDate = model.NormDate(in.OrderDate)
	now := model.Now()

	err := s.Tx(ctx, func(tx *sql.Tx) error {
		customer, err := loadPartnerName(ctx, tx, in.CustomerID)
		if err != nil {
			return err
		}
		wh, err := loadWarehouseName(ctx, tx, in.WarehouseID)
		if err != nil {
			return err
		}
		t := calcTotals(in.Items, in.DiscountAmount, in.OtherFee)

		if id > 0 {
			var status int
			if err := tx.QueryRowContext(ctx, `SELECT status FROM sales_orders WHERE id=?`, id).Scan(&status); err != nil {
				if err == sql.ErrNoRows {
					return ErrNotFound
				}
				return err
			}
			if status != model.DocDraft {
				return fmt.Errorf("%w: 只有草稿状态的单据才能修改", ErrBadRequest)
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE sales_orders SET customer_id=?, customer_name=?, warehouse_id=?, warehouse_name=?,
				   order_date=?, total_qty=?, total_amount=?, discount_amount=?, other_fee=?, settle_amount=?,
				   remark=?, updated_at=? WHERE id=?`,
				in.CustomerID, customer, in.WarehouseID, wh, in.OrderDate,
				float64(t.TotalQty), t.TotalAmount, t.DiscountAmount, t.OtherFee, t.SettleAmount,
				in.Remark, now, id); err != nil {
				return wrapErr(err, "update sales")
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM sales_items WHERE order_id=?`, id); err != nil {
				return err
			}
			return insertSalesItems(ctx, tx, id, in.Items, nil)
		}

		no, err := nextDocNo(ctx, tx, "sales", "XS", in.OrderDate)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO sales_orders(order_no, customer_id, customer_name, warehouse_id, warehouse_name,
			   order_date, status, total_qty, total_amount, cost_amount, discount_amount, other_fee,
			   settle_amount, received_amount, remark, created_by, created_by_name, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,0,?,?,0,?,?,?,0,?,?,?,?,?)`,
			no, in.CustomerID, customer, in.WarehouseID, wh, in.OrderDate,
			float64(t.TotalQty), t.TotalAmount, t.DiscountAmount, t.OtherFee, t.SettleAmount,
			in.Remark, user.ID, user.Name, now, now)
		if err != nil {
			return wrapErr(err, "insert sales")
		}
		id, _ = res.LastInsertId()
		return insertSalesItems(ctx, tx, id, in.Items, nil)
	})
	return id, err
}

func insertSalesItems(ctx context.Context, tx *sql.Tx, orderID int64, items []OrderItemInput, costs []model.Price) error {
	for i, it := range items {
		b, err := loadProductBrief(ctx, tx, it.ProductID)
		if err != nil {
			return err
		}
		amount := model.AmountOf(it.Qty, it.Price)
		var cost model.Price
		if costs != nil && i < len(costs) {
			cost = costs[i]
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sales_items(order_id, product_id, product_code, product_name, spec, unit,
			   qty, price, amount, cost_price, remark, sort) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
			orderID, it.ProductID, b.Code, b.Name, b.Spec, b.Unit,
			float64(it.Qty), it.Price, amount, cost, it.Remark, i); err != nil {
			return err
		}
	}
	return nil
}

// ConfirmSales 销售出库确认：扣减库存并记录成本。
func (s *Store) ConfirmSales(ctx context.Context, id int64, user OpUser) error {
	strict := !s.SettingBool(ctx, "allow_negative_stock", false)
	followPrice := s.SettingBool(ctx, "sales_price_follow", false)
	return s.Tx(ctx, func(tx *sql.Tx) error {
		o, ok, err := queryRow[model.SalesOrder](ctx, tx,
			`SELECT `+salesCols+` FROM sales_orders o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if o.Status == model.DocConfirmed {
			return fmt.Errorf("%w: 该销售单已出库", ErrBadRequest)
		}
		if o.Status == model.DocVoided {
			return fmt.Errorf("%w: 该销售单已作废", ErrBadRequest)
		}
		items, err := query[model.OrderItem](ctx, tx,
			`SELECT id, order_id AS doc_id, product_id, product_code, product_name, spec, unit,
			   qty, price, amount, remark, 0 AS sort FROM sales_items WHERE order_id=? ORDER BY id`, id)
		if err != nil {
			return err
		}
		var costTotal model.Money
		for _, it := range items {
			res, err := applyStock(ctx, tx, stockMoveReq{
				WarehouseID: o.WarehouseID,
				ProductID:   it.ProductID,
				Qty:         model.Qty(-it.Qty.Float()),
				BizType:     "sales",
				BizID:       o.ID,
				BizNo:       o.OrderNo,
				User:        user,
				Strict:      strict,
			})
			if err != nil {
				return err
			}
			costTotal += res.Amount
			if _, err := tx.ExecContext(ctx,
				`UPDATE sales_items SET cost_price=? WHERE id=?`, res.CostPrice, it.ID); err != nil {
				return err
			}
			if followPrice && it.Price > 0 {
				if err := updateProductRefPrice(ctx, tx, it.ProductID, it.Price, false); err != nil {
					return err
				}
			}
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE sales_orders SET status=1, cost_amount=?, confirmed_at=?, confirmed_by=?, updated_at=? WHERE id=?`,
			costTotal, model.Now(), user.ID, model.Now(), id)
		return err
	})
}

// VoidSales 作废销售单（已出库的先冲回库存）。
func (s *Store) VoidSales(ctx context.Context, id int64, user OpUser) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		o, ok, err := queryRow[model.SalesOrder](ctx, tx,
			`SELECT `+salesCols+` FROM sales_orders o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if o.Status == model.DocVoided {
			return fmt.Errorf("%w: 该销售单已作废", ErrBadRequest)
		}
		if o.ReceivedAmount > 0 {
			return fmt.Errorf("%w: 该销售单已有收款记录，请先作废相关收款单", ErrBadRequest)
		}
		if o.Status == model.DocConfirmed {
			if err := reverseMovesByBiz(ctx, tx, "sales", id, user); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE sales_orders SET status=2, updated_at=? WHERE id=?`, model.Now(), id)
		return err
	})
}

// DeleteSales 删除销售单（仅草稿）。
func (s *Store) DeleteSales(ctx context.Context, id int64) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		var status int
		if err := tx.QueryRowContext(ctx, `SELECT status FROM sales_orders WHERE id=?`, id).Scan(&status); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		if status != model.DocDraft {
			return fmt.Errorf("%w: 只有草稿状态的单据才能删除，已出库请先作废", ErrBadRequest)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM sales_items WHERE order_id=?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM sales_orders WHERE id=?`, id)
		return err
	})
}

// ---------------------------------------------------------------------------
// 辅助
// ---------------------------------------------------------------------------

func docFilter(keyword, from, to string, status int) ([]string, []any) {
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(keyword); kw != "" {
		where = append(where, "(o.order_no LIKE ? OR o.supplier_name LIKE ? OR o.customer_name LIKE ? OR o.warehouse_name LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	if from != "" {
		where = append(where, "o.order_date >= ?")
		args = append(args, from)
	}
	if to != "" {
		where = append(where, "o.order_date <= ?")
		args = append(args, to)
	}
	if status >= 0 && status <= 2 {
		where = append(where, "o.status=?")
		args = append(args, status)
	}
	return where, args
}

func loadPartnerName(ctx context.Context, tx *sql.Tx, id int64) (string, error) {
	if id == 0 {
		return "", fmt.Errorf("%w: 请选择往来单位", ErrBadRequest)
	}
	var name string
	err := tx.QueryRowContext(ctx, `SELECT name FROM partners WHERE id=?`, id).Scan(&name)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("%w: 往来单位不存在", ErrBadRequest)
	}
	return name, err
}

func loadWarehouseName(ctx context.Context, tx *sql.Tx, id int64) (string, error) {
	if id == 0 {
		wh, ok, err := queryRow[model.Warehouse](ctx, tx,
			`SELECT id, name FROM warehouses WHERE status=1 ORDER BY is_default DESC, sort, id LIMIT 1`)
		if err != nil {
			return "", err
		}
		if !ok || wh.ID == 0 {
			return "", fmt.Errorf("%w: 请先建立仓库", ErrBadRequest)
		}
		return wh.Name, nil
	}
	var name string
	err := tx.QueryRowContext(ctx, `SELECT name FROM warehouses WHERE id=?`, id).Scan(&name)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("%w: 仓库不存在", ErrBadRequest)
	}
	return name, err
}
