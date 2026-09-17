package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"mufish/internal/model"
)

// ---------------------------------------------------------------------------
// 库存查询
// ---------------------------------------------------------------------------

// StockQuery 库存查询条件。
type StockQuery struct {
	Keyword     string
	WarehouseID int64
	CategoryID  int64
	OnlyLow     bool // 仅显示低于安全库存
	OnlyStock   bool // 仅显示有库存
	Page        int
	Size        int
}

// ListStocks 分页查询库存余额。
func (s *Store) ListStocks(ctx context.Context, q StockQuery) (model.Page[model.Stock], error) {
	var out model.Page[model.Stock]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(p.code LIKE ? OR p.name LIKE ? OR p.spec LIKE ? OR p.barcode LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	if q.WarehouseID > 0 {
		where = append(where, "st.warehouse_id=?")
		args = append(args, q.WarehouseID)
	}
	if q.CategoryID > 0 {
		where = append(where, "(p.category_id=? OR p.category_id IN (SELECT id FROM categories WHERE parent_id=?))")
		args = append(args, q.CategoryID, q.CategoryID)
	}
	if q.OnlyStock {
		where = append(where, "st.qty <> 0")
	}
	if q.OnlyLow {
		where = append(where, "p.safety_stock > 0 AND st.qty < p.safety_stock")
	}
	cond := strings.Join(where, " AND ")

	base := `FROM stocks st
		JOIN products p ON p.id = st.product_id
		JOIN warehouses w ON w.id = st.warehouse_id
		LEFT JOIN categories c ON c.id = p.category_id
		WHERE ` + cond

	total, err := countOf(ctx, s.db, `SELECT COUNT(*) `+base, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.Stock](ctx, s.db,
		`SELECT st.id, st.warehouse_id, w.name AS warehouse_name, st.product_id, p.code AS product_code,
		   p.name AS product_name, p.spec, p.unit, COALESCE(c.name,'') AS category_name,
		   st.qty, st.cost_price, st.amount, p.safety_stock, st.updated_at `+base+
			` ORDER BY p.code, w.sort, w.id LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.Stock]{List: list, Total: total, Page: page, Size: size}, nil
}

// StockLedgerQuery 库存流水查询条件。
type StockLedgerQuery struct {
	Keyword     string
	WarehouseID int64
	ProductID   int64
	BizType     string
	From        string
	To          string
	Page        int
	Size        int
}

// ListStockMoves 分页查询库存流水。
func (s *Store) ListStockMoves(ctx context.Context, q StockLedgerQuery) (model.Page[model.StockMove], error) {
	var out model.Page[model.StockMove]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(p.code LIKE ? OR p.name LIKE ? OR m.biz_no LIKE ? OR m.remark LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	if q.WarehouseID > 0 {
		where = append(where, "m.warehouse_id=?")
		args = append(args, q.WarehouseID)
	}
	if q.ProductID > 0 {
		where = append(where, "m.product_id=?")
		args = append(args, q.ProductID)
	}
	if q.BizType != "" {
		where = append(where, "m.biz_type=?")
		args = append(args, q.BizType)
	}
	if q.From != "" {
		where = append(where, "m.created_at >= ?")
		args = append(args, q.From+" 00:00:00")
	}
	if q.To != "" {
		where = append(where, "m.created_at <= ?")
		args = append(args, q.To+" 23:59:59")
	}
	cond := strings.Join(where, " AND ")

	base := `FROM stock_moves m
		JOIN products p ON p.id=m.product_id
		JOIN warehouses w ON w.id=m.warehouse_id
		WHERE ` + cond
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) `+base, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.StockMove](ctx, s.db,
		`SELECT m.id, m.warehouse_id, w.name AS warehouse_name, m.product_id, p.code AS product_code,
		   p.name AS product_name, p.spec, p.unit, m.biz_type, m.biz_id, m.biz_no, m.direction,
		   m.qty, m.price, m.amount, m.balance_after, m.remark, m.created_by, m.created_by_name, m.created_at `+
			base+` ORDER BY m.id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.StockMove]{List: list, Total: total, Page: page, Size: size}, nil
}

// LowStockList 库存预警列表。
func (s *Store) LowStockList(ctx context.Context, warehouseID int64, limit int) ([]model.Stock, error) {
	if limit <= 0 {
		limit = 100
	}
	args := []any{}
	cond := "p.safety_stock > 0 AND p.status=1"
	if warehouseID > 0 {
		cond += " AND st.warehouse_id=?"
		args = append(args, warehouseID)
	}
	args = append(args, limit)
	return query[model.Stock](ctx, s.db,
		`SELECT st.id, st.warehouse_id, w.name AS warehouse_name, st.product_id, p.code AS product_code,
		   p.name AS product_name, p.spec, p.unit, '' AS category_name,
		   st.qty, st.cost_price, st.amount, p.safety_stock, st.updated_at
		 FROM stocks st
		 JOIN products p ON p.id=st.product_id
		 JOIN warehouses w ON w.id=st.warehouse_id
		 WHERE `+cond+` AND st.qty < p.safety_stock
		 ORDER BY (p.safety_stock - st.qty) DESC LIMIT ?`, args...)
}

// ProductStockByWarehouse 查询某商品在各仓库的库存。
func (s *Store) ProductStockByWarehouse(ctx context.Context, productID int64) ([]model.Stock, error) {
	return query[model.Stock](ctx, s.db,
		`SELECT st.id, st.warehouse_id, w.name AS warehouse_name, st.product_id, p.code AS product_code,
		   p.name AS product_name, p.spec, p.unit, '' AS category_name,
		   st.qty, st.cost_price, st.amount, p.safety_stock, st.updated_at
		 FROM stocks st JOIN products p ON p.id=st.product_id JOIN warehouses w ON w.id=st.warehouse_id
		 WHERE st.product_id=? ORDER BY w.sort, w.id`, productID)
}

// ---------------------------------------------------------------------------
// 库存盘点
// ---------------------------------------------------------------------------

// StockTakeQuery 盘点单查询条件。
type StockTakeQuery struct {
	Keyword     string
	WarehouseID int64
	Status      int
	From        string
	To          string
	Page        int
	Size        int
}

const takeCols = `o.id, o.take_no, o.warehouse_id, o.warehouse_name, o.take_date, o.status,
	o.diff_qty, o.diff_amount, o.remark, o.created_by, o.created_by_name, o.finished_at, o.created_at`

// ListStockTakes 分页查询盘点单。
func (s *Store) ListStockTakes(ctx context.Context, q StockTakeQuery) (model.Page[model.StockTake], error) {
	var out model.Page[model.StockTake]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(o.take_no LIKE ? OR o.warehouse_name LIKE ? OR o.remark LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	if q.WarehouseID > 0 {
		where = append(where, "o.warehouse_id=?")
		args = append(args, q.WarehouseID)
	}
	if q.From != "" {
		where = append(where, "o.take_date >= ?")
		args = append(args, q.From)
	}
	if q.To != "" {
		where = append(where, "o.take_date <= ?")
		args = append(args, q.To)
	}
	if q.Status >= 0 && q.Status <= 2 {
		where = append(where, "o.status=?")
		args = append(args, q.Status)
	}
	cond := strings.Join(where, " AND ")
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM stock_takes o WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.StockTake](ctx, s.db,
		`SELECT `+takeCols+` FROM stock_takes o WHERE `+cond+` ORDER BY o.id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.StockTake]{List: list, Total: total, Page: page, Size: size}, nil
}

// GetStockTake 获取盘点单（含明细）。
func (s *Store) GetStockTake(ctx context.Context, id int64) (model.StockTake, error) {
	o, ok, err := queryRow[model.StockTake](ctx, s.db,
		`SELECT `+takeCols+` FROM stock_takes o WHERE o.id=?`, id)
	if err != nil {
		return o, err
	}
	if !ok {
		return o, ErrNotFound
	}
	items, err := query[model.StockTakeItem](ctx, s.db,
		`SELECT id, take_id, product_id, product_code, product_name, spec, unit,
		   book_qty, real_qty, diff_qty, price, diff_amount, remark
		 FROM stock_take_items WHERE take_id=? ORDER BY id`, id)
	if err != nil {
		return o, err
	}
	o.Items = items
	return o, nil
}

// StockTakeItemInput 盘点明细入参。
type StockTakeItemInput struct {
	ProductID int64     `json:"product_id"`
	RealQty   model.Qty `json:"real_qty"`
	Remark    string    `json:"remark"`
}

// StockTakeInput 盘点单入参。
type StockTakeInput struct {
	WarehouseID int64                `json:"warehouse_id"`
	TakeDate    string               `json:"take_date"`
	Remark      string               `json:"remark"`
	Items       []StockTakeItemInput `json:"items"`
}

// SaveStockTake 新建或修改盘点单（仅草稿）。
func (s *Store) SaveStockTake(ctx context.Context, id int64, in StockTakeInput, user OpUser) (int64, error) {
	if len(in.Items) == 0 {
		return 0, fmt.Errorf("%w: 请至少录入一行盘点明细", ErrBadRequest)
	}
	in.TakeDate = model.NormDate(in.TakeDate)
	now := model.Now()
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		wh, err := loadWarehouseName(ctx, tx, in.WarehouseID)
		if err != nil {
			return err
		}
		if id == 0 {
			no, err := nextDocNo(ctx, tx, "take", "PD", in.TakeDate)
			if err != nil {
				return err
			}
			res, err := tx.ExecContext(ctx,
				`INSERT INTO stock_takes(take_no, warehouse_id, warehouse_name, take_date, status, remark,
				   created_by, created_by_name, created_at) VALUES(?,?,?,?,0,?,?,?,?)`,
				no, in.WarehouseID, wh, in.TakeDate, in.Remark, user.ID, user.Name, now)
			if err != nil {
				return wrapErr(err, "insert stock take")
			}
			id, _ = res.LastInsertId()
		} else {
			var status int
			if err := tx.QueryRowContext(ctx, `SELECT status FROM stock_takes WHERE id=?`, id).Scan(&status); err != nil {
				if err == sql.ErrNoRows {
					return ErrNotFound
				}
				return err
			}
			if status != model.DocDraft {
				return fmt.Errorf("%w: 只有草稿状态的盘点单才能修改", ErrBadRequest)
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE stock_takes SET warehouse_id=?, warehouse_name=?, take_date=?, remark=? WHERE id=?`,
				in.WarehouseID, wh, in.TakeDate, in.Remark, id); err != nil {
				return wrapErr(err, "update stock take")
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM stock_take_items WHERE take_id=?`, id); err != nil {
				return err
			}
		}

		var diffQty model.Qty
		var diffAmount model.Money
		for _, it := range in.Items {
			if it.ProductID == 0 {
				return fmt.Errorf("%w: 存在未选择商品的明细行", ErrBadRequest)
			}
			b, err := loadProductBrief(ctx, tx, it.ProductID)
			if err != nil {
				return err
			}
			var bookQty float64
			var price int64
			err = tx.QueryRowContext(ctx,
				`SELECT qty, cost_price FROM stocks WHERE warehouse_id=? AND product_id=?`,
				in.WarehouseID, it.ProductID).Scan(&bookQty, &price)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if price == 0 {
				_ = tx.QueryRowContext(ctx, `SELECT purchase_price FROM products WHERE id=?`, it.ProductID).Scan(&price)
			}
			dq := round4(it.RealQty.Float() - bookQty)
			da := model.Money(int64(round4(float64(price) * dq / 100)))
			diffQty += model.Qty(dq)
			diffAmount += da
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO stock_take_items(take_id, product_id, product_code, product_name, spec, unit,
				   book_qty, real_qty, diff_qty, price, diff_amount, remark) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
				id, it.ProductID, b.Code, b.Name, b.Spec, b.Unit,
				bookQty, float64(it.RealQty), dq, price, da, it.Remark); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE stock_takes SET diff_qty=?, diff_amount=? WHERE id=?`,
			float64(diffQty), diffAmount, id)
		return err
	})
	return id, err
}

// FinishStockTake 完成盘点：按实盘数调整库存。
func (s *Store) FinishStockTake(ctx context.Context, id int64, user OpUser) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		o, ok, err := queryRow[model.StockTake](ctx, tx,
			`SELECT `+takeCols+` FROM stock_takes o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if o.Status != model.DocDraft {
			return fmt.Errorf("%w: 该盘点单已处理", ErrBadRequest)
		}
		items, err := query[model.StockTakeItem](ctx, tx,
			`SELECT id, take_id, product_id, product_code, product_name, spec, unit,
			   book_qty, real_qty, diff_qty, price, diff_amount, remark
			 FROM stock_take_items WHERE take_id=? ORDER BY id`, id)
		if err != nil {
			return err
		}
		for _, it := range items {
			if it.DiffQty == 0 {
				continue
			}
			if _, err := applyStock(ctx, tx, stockMoveReq{
				WarehouseID: o.WarehouseID,
				ProductID:   it.ProductID,
				Qty:         it.DiffQty,
				Price:       it.Price,
				BizType:     "take",
				BizID:       o.ID,
				BizNo:       o.TakeNo,
				Remark:      "盘点调整",
				User:        user,
			}); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE stock_takes SET status=1, finished_at=? WHERE id=?`, model.Now(), id)
		return err
	})
}

// VoidStockTake 作废盘点单（已完成则回滚调整）。
func (s *Store) VoidStockTake(ctx context.Context, id int64, user OpUser) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		o, ok, err := queryRow[model.StockTake](ctx, tx,
			`SELECT `+takeCols+` FROM stock_takes o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if o.Status == model.DocVoided {
			return fmt.Errorf("%w: 该盘点单已作废", ErrBadRequest)
		}
		if o.Status == model.DocConfirmed {
			if err := reverseMovesByBiz(ctx, tx, "take", id, user); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE stock_takes SET status=2 WHERE id=?`, id)
		return err
	})
}

// DeleteStockTake 删除盘点单（仅草稿）。
func (s *Store) DeleteStockTake(ctx context.Context, id int64) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		var status int
		if err := tx.QueryRowContext(ctx, `SELECT status FROM stock_takes WHERE id=?`, id).Scan(&status); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		if status != model.DocDraft {
			return fmt.Errorf("%w: 只有草稿状态的盘点单才能删除", ErrBadRequest)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM stock_take_items WHERE take_id=?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM stock_takes WHERE id=?`, id)
		return err
	})
}

// ---------------------------------------------------------------------------
// 库存调拨
// ---------------------------------------------------------------------------

// TransferQuery 调拨单查询条件。
type TransferQuery struct {
	Keyword string
	Status  int
	From    string
	To      string
	Page    int
	Size    int
}

const transferCols = `o.id, o.transfer_no, o.from_warehouse_id, o.from_name, o.to_warehouse_id, o.to_name,
	o.transfer_date, o.status, o.total_qty, o.total_amount, o.remark, o.created_by, o.created_by_name,
	o.confirmed_at, o.created_at`

// ListTransfers 分页查询调拨单。
func (s *Store) ListTransfers(ctx context.Context, q TransferQuery) (model.Page[model.Transfer], error) {
	var out model.Page[model.Transfer]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(o.transfer_no LIKE ? OR o.from_name LIKE ? OR o.to_name LIKE ? OR o.remark LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	if q.From != "" {
		where = append(where, "o.transfer_date >= ?")
		args = append(args, q.From)
	}
	if q.To != "" {
		where = append(where, "o.transfer_date <= ?")
		args = append(args, q.To)
	}
	if q.Status >= 0 && q.Status <= 2 {
		where = append(where, "o.status=?")
		args = append(args, q.Status)
	}
	cond := strings.Join(where, " AND ")
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM transfers o WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.Transfer](ctx, s.db,
		`SELECT `+transferCols+` FROM transfers o WHERE `+cond+` ORDER BY o.id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.Transfer]{List: list, Total: total, Page: page, Size: size}, nil
}

// GetTransfer 获取调拨单（含明细）。
func (s *Store) GetTransfer(ctx context.Context, id int64) (model.Transfer, error) {
	o, ok, err := queryRow[model.Transfer](ctx, s.db,
		`SELECT `+transferCols+` FROM transfers o WHERE o.id=?`, id)
	if err != nil {
		return o, err
	}
	if !ok {
		return o, ErrNotFound
	}
	items, err := query[model.TransferItem](ctx, s.db,
		`SELECT id, transfer_id, product_id, product_code, product_name, spec, unit, qty, price, amount
		 FROM transfer_items WHERE transfer_id=? ORDER BY id`, id)
	if err != nil {
		return o, err
	}
	o.Items = items
	return o, nil
}

// TransferInput 调拨单入参。
type TransferInput struct {
	FromWarehouseID int64            `json:"from_warehouse_id"`
	ToWarehouseID   int64            `json:"to_warehouse_id"`
	TransferDate    string           `json:"transfer_date"`
	Remark          string           `json:"remark"`
	Items           []OrderItemInput `json:"items"`
}

// SaveTransfer 新建或修改调拨单。
func (s *Store) SaveTransfer(ctx context.Context, id int64, in TransferInput, user OpUser) (int64, error) {
	if in.FromWarehouseID == 0 || in.ToWarehouseID == 0 {
		return 0, fmt.Errorf("%w: 请选择调出与调入仓库", ErrBadRequest)
	}
	if in.FromWarehouseID == in.ToWarehouseID {
		return 0, fmt.Errorf("%w: 调出仓库与调入仓库不能相同", ErrBadRequest)
	}
	if len(in.Items) == 0 {
		return 0, fmt.Errorf("%w: 请至少录入一行调拨明细", ErrBadRequest)
	}
	for i, it := range in.Items {
		if it.ProductID == 0 {
			return 0, fmt.Errorf("%w: 第 %d 行未选择商品", ErrBadRequest, i+1)
		}
		if it.Qty <= 0 {
			return 0, fmt.Errorf("%w: 第 %d 行数量必须大于 0", ErrBadRequest, i+1)
		}
	}
	in.TransferDate = model.NormDate(in.TransferDate)
	now := model.Now()
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		from, err := loadWarehouseName(ctx, tx, in.FromWarehouseID)
		if err != nil {
			return err
		}
		to, err := loadWarehouseName(ctx, tx, in.ToWarehouseID)
		if err != nil {
			return err
		}
		var totalQty model.Qty
		var totalAmount model.Money
		for _, it := range in.Items {
			totalQty += it.Qty
			totalAmount += model.AmountOf(it.Qty, it.Price)
		}
		if id == 0 {
			no, err := nextDocNo(ctx, tx, "transfer", "DB", in.TransferDate)
			if err != nil {
				return err
			}
			res, err := tx.ExecContext(ctx,
				`INSERT INTO transfers(transfer_no, from_warehouse_id, from_name, to_warehouse_id, to_name,
				   transfer_date, status, total_qty, total_amount, remark, created_by, created_by_name, created_at)
				 VALUES(?,?,?,?,?,?,0,?,?,?,?,?,?)`,
				no, in.FromWarehouseID, from, in.ToWarehouseID, to, in.TransferDate,
				float64(totalQty), totalAmount, in.Remark, user.ID, user.Name, now)
			if err != nil {
				return wrapErr(err, "insert transfer")
			}
			id, _ = res.LastInsertId()
		} else {
			var status int
			if err := tx.QueryRowContext(ctx, `SELECT status FROM transfers WHERE id=?`, id).Scan(&status); err != nil {
				if err == sql.ErrNoRows {
					return ErrNotFound
				}
				return err
			}
			if status != model.DocDraft {
				return fmt.Errorf("%w: 只有草稿状态的调拨单才能修改", ErrBadRequest)
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE transfers SET from_warehouse_id=?, from_name=?, to_warehouse_id=?, to_name=?,
				   transfer_date=?, total_qty=?, total_amount=?, remark=? WHERE id=?`,
				in.FromWarehouseID, from, in.ToWarehouseID, to, in.TransferDate,
				float64(totalQty), totalAmount, in.Remark, id); err != nil {
				return wrapErr(err, "update transfer")
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM transfer_items WHERE transfer_id=?`, id); err != nil {
				return err
			}
		}
		for _, it := range in.Items {
			b, err := loadProductBrief(ctx, tx, it.ProductID)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO transfer_items(transfer_id, product_id, product_code, product_name, spec, unit, qty, price, amount)
				 VALUES(?,?,?,?,?,?,?,?,?)`,
				id, it.ProductID, b.Code, b.Name, b.Spec, b.Unit,
				float64(it.Qty), it.Price, model.AmountOf(it.Qty, it.Price)); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

// ConfirmTransfer 确认调拨：调出仓减库存，调入仓加库存（按调出仓成本结转）。
func (s *Store) ConfirmTransfer(ctx context.Context, id int64, user OpUser) error {
	strict := !s.SettingBool(ctx, "allow_negative_stock", false)
	return s.Tx(ctx, func(tx *sql.Tx) error {
		o, ok, err := queryRow[model.Transfer](ctx, tx,
			`SELECT `+transferCols+` FROM transfers o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if o.Status == model.DocConfirmed {
			return fmt.Errorf("%w: 该调拨单已确认", ErrBadRequest)
		}
		if o.Status == model.DocVoided {
			return fmt.Errorf("%w: 该调拨单已作废", ErrBadRequest)
		}
		items, err := query[model.TransferItem](ctx, tx,
			`SELECT id, transfer_id, product_id, product_code, product_name, spec, unit, qty, price, amount
			 FROM transfer_items WHERE transfer_id=? ORDER BY id`, id)
		if err != nil {
			return err
		}
		for _, it := range items {
			out, err := applyStock(ctx, tx, stockMoveReq{
				WarehouseID: o.FromWarehouseID,
				ProductID:   it.ProductID,
				Qty:         model.Qty(-it.Qty.Float()),
				BizType:     "transfer_out",
				BizID:       o.ID,
				BizNo:       o.TransferNo,
				Remark:      "调拨至 " + o.ToName,
				User:        user,
				Strict:      strict,
			})
			if err != nil {
				return err
			}
			if _, err := applyStock(ctx, tx, stockMoveReq{
				WarehouseID: o.ToWarehouseID,
				ProductID:   it.ProductID,
				Qty:         it.Qty,
				Price:       out.CostPrice,
				BizType:     "transfer_in",
				BizID:       o.ID,
				BizNo:       o.TransferNo,
				Remark:      "调拨自 " + o.FromName,
				User:        user,
			}); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE transfer_items SET price=? WHERE id=?`, out.CostPrice, it.ID); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE transfers SET status=1, confirmed_at=? WHERE id=?`, model.Now(), id)
		return err
	})
}

// VoidTransfer 作废调拨单。
func (s *Store) VoidTransfer(ctx context.Context, id int64, user OpUser) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		o, ok, err := queryRow[model.Transfer](ctx, tx,
			`SELECT `+transferCols+` FROM transfers o WHERE o.id=?`, id)
		if err != nil {
			return err
		}
		if !ok {
			return ErrNotFound
		}
		if o.Status == model.DocVoided {
			return fmt.Errorf("%w: 该调拨单已作废", ErrBadRequest)
		}
		if o.Status == model.DocConfirmed {
			if err := reverseMovesByBiz(ctx, tx, "transfer_out", id, user); err != nil {
				return err
			}
			if err := reverseMovesByBiz(ctx, tx, "transfer_in", id, user); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `UPDATE transfers SET status=2 WHERE id=?`, id)
		return err
	})
}

// DeleteTransfer 删除调拨单（仅草稿）。
func (s *Store) DeleteTransfer(ctx context.Context, id int64) error {
	return s.Tx(ctx, func(tx *sql.Tx) error {
		var status int
		if err := tx.QueryRowContext(ctx, `SELECT status FROM transfers WHERE id=?`, id).Scan(&status); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		if status != model.DocDraft {
			return fmt.Errorf("%w: 只有草稿状态的调拨单才能删除", ErrBadRequest)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM transfer_items WHERE transfer_id=?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM transfers WHERE id=?`, id)
		return err
	})
}
