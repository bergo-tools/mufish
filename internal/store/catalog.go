package store

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"mufish/internal/model"
)

// ---------------------------------------------------------------------------
// 商品分类
// ---------------------------------------------------------------------------

// ListCategories 查询分类（含树形 children）。
func (s *Store) ListCategories(ctx context.Context) ([]model.Category, error) {
	flat, err := query[model.Category](ctx, s.db,
		`SELECT id, name, parent_id, sort, remark FROM categories ORDER BY sort, id`)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*model.Category, len(flat))
	for i := range flat {
		byID[flat[i].ID] = &flat[i]
	}
	// 先完成所有父子链接，再复制根节点。
	// 若边遍历边复制，父节点排在子节点之前时会丢失 children。
	roots := make([]*model.Category, 0, len(flat))
	for i := range flat {
		c := &flat[i]
		if c.ParentID != 0 {
			if p, ok := byID[c.ParentID]; ok {
				p.Children = append(p.Children, c)
				continue
			}
		}
		roots = append(roots, c)
	}
	out := make([]model.Category, 0, len(roots))
	for _, r := range roots {
		out = append(out, *r)
	}
	return out, nil
}

// AllCategories 查询扁平分类列表。
func (s *Store) AllCategories(ctx context.Context) ([]model.Category, error) {
	return query[model.Category](ctx, s.db,
		`SELECT id, name, parent_id, sort, remark FROM categories ORDER BY sort, id`)
}

// CategoryInput 分类入参。
type CategoryInput struct {
	Name     string `json:"name"`
	ParentID int64  `json:"parent_id"`
	Sort     int    `json:"sort"`
	Remark   string `json:"remark"`
}

// categoryTreeCond 返回「分类自身及其全部后代」的 SQL 条件。
// column 只传内部常量，不能来自用户输入。
func categoryTreeCond(column string, id int64) (string, []any) {
	return column + ` IN (
		WITH RECURSIVE sub(id) AS (
			SELECT id FROM categories WHERE id=?
			UNION
			SELECT c.id FROM categories c JOIN sub s ON c.parent_id=s.id
		)
		SELECT id FROM sub
	)`, []any{id}
}

// SaveCategory 新建或更新分类。
func (s *Store) SaveCategory(ctx context.Context, id int64, in CategoryInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return 0, fmt.Errorf("%w: 分类名称不能为空", ErrBadRequest)
	}
	if id > 0 && in.ParentID == id {
		return 0, fmt.Errorf("%w: 上级分类不能是自己", ErrBadRequest)
	}

	outID := id
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		if in.ParentID > 0 {
			var parent int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE id=?`, in.ParentID).Scan(&parent); err != nil {
				return err
			}
			if parent == 0 {
				return fmt.Errorf("%w: 上级分类不存在", ErrBadRequest)
			}
		}
		if id > 0 {
			// 不能把父级设置成自己的任意后代，否则分类树会形成环。
			if in.ParentID > 0 {
				var descendants int
				if err := tx.QueryRowContext(ctx,
					`WITH RECURSIVE sub(id) AS (
					   SELECT id FROM categories WHERE parent_id=?
					   UNION
					   SELECT c.id FROM categories c JOIN sub s ON c.parent_id=s.id
					 ) SELECT COUNT(*) FROM sub WHERE id=?`, id, in.ParentID).Scan(&descendants); err != nil {
					return err
				}
				if descendants > 0 {
					return fmt.Errorf("%w: 上级分类不能是自己的下级", ErrBadRequest)
				}
			}
			res, err := tx.ExecContext(ctx,
				`UPDATE categories SET name=?, parent_id=?, sort=?, remark=? WHERE id=?`,
				in.Name, in.ParentID, in.Sort, in.Remark, id)
			if err != nil {
				return wrapErr(err, "update category")
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return ErrNotFound
			}
			return nil
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO categories(name, parent_id, sort, remark) VALUES(?,?,?,?)`,
			in.Name, in.ParentID, in.Sort, in.Remark)
		if err != nil {
			return wrapErr(err, "insert category")
		}
		outID, _ = res.LastInsertId()
		return nil
	})
	return outID, err
}

// DeleteCategory 删除分类。
func (s *Store) DeleteCategory(ctx context.Context, id int64) error {
	var child int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE parent_id=?`, id).Scan(&child); err != nil {
		return err
	}
	if child > 0 {
		return fmt.Errorf("%w: 请先删除下级分类", ErrInUse)
	}
	var used int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM products WHERE category_id=?`, id).Scan(&used); err != nil {
		return err
	}
	if used > 0 {
		return fmt.Errorf("%w: 有 %d 个商品使用该分类", ErrInUse, used)
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM categories WHERE id=?`, id)
	if err != nil {
		return wrapErr(err, "delete category")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// 商品
// ---------------------------------------------------------------------------

const productCols = `p.id, p.code, p.name, p.spec, p.unit, p.category_id,
	COALESCE(c.name,'') AS category_name, p.barcode, p.brand, p.purchase_price, p.sale_price,
	p.safety_stock, p.safety_stock_max, p.status, p.remark, p.created_at, p.updated_at`

// ProductQuery 商品查询条件。
type ProductQuery struct {
	Keyword    string
	CategoryID int64
	Status     int // -1 全部
	Page       int
	Size       int
}

// ListProducts 分页查询商品。
func (s *Store) ListProducts(ctx context.Context, q ProductQuery) (model.Page[model.Product], error) {
	var out model.Page[model.Product]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(p.code LIKE ? OR p.name LIKE ? OR p.spec LIKE ? OR p.barcode LIKE ? OR p.brand LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like, like)
	}
	if q.CategoryID > 0 {
		cond, cargs := categoryTreeCond("p.category_id", q.CategoryID)
		where = append(where, cond)
		args = append(args, cargs...)
	}
	if q.Status == 0 || q.Status == 1 {
		where = append(where, "p.status=?")
		args = append(args, q.Status)
	}
	cond := strings.Join(where, " AND ")
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM products p WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.Product](ctx, s.db,
		`SELECT `+productCols+`,
		   COALESCE((SELECT SUM(s.qty) FROM stocks s WHERE s.product_id=p.id),0) AS stock_qty
		 FROM products p LEFT JOIN categories c ON c.id=p.category_id
		 WHERE `+cond+` ORDER BY p.id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.Product]{List: list, Total: total, Page: page, Size: size}, nil
}

// AllProducts 查询全部启用商品（用于下拉选择）。
func (s *Store) AllProducts(ctx context.Context, keyword string) ([]model.Product, error) {
	args := []any{}
	cond := "p.status=1"
	if kw := strings.TrimSpace(keyword); kw != "" {
		cond += " AND (p.code LIKE ? OR p.name LIKE ? OR p.barcode LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	return query[model.Product](ctx, s.db,
		`SELECT `+productCols+` FROM products p LEFT JOIN categories c ON c.id=p.category_id
		 WHERE `+cond+` ORDER BY p.code LIMIT 300`, args...)
}

// AllProductsForWarehouse 查询全部启用商品，并带出指定仓库的结存数量。
// 销售开单需要在下拉/明细中显示“当前仓库可用库存”。
func (s *Store) AllProductsForWarehouse(ctx context.Context, keyword string, warehouseID int64) ([]model.Product, error) {
	args := []any{warehouseID}
	cond := "p.status=1"
	if kw := strings.TrimSpace(keyword); kw != "" {
		cond += " AND (p.code LIKE ? OR p.name LIKE ? OR p.barcode LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like, like)
	}
	return query[model.Product](ctx, s.db,
		`SELECT `+productCols+`,
		   COALESCE((SELECT SUM(s.qty) FROM stocks s
		             WHERE s.product_id=p.id AND s.warehouse_id=?),0) AS stock_qty
		 FROM products p LEFT JOIN categories c ON c.id=p.category_id
		 WHERE `+cond+` ORDER BY p.code LIMIT 300`, args...)
}

// GetProduct 按 ID 获取商品。
func (s *Store) GetProduct(ctx context.Context, id int64) (model.Product, error) {
	p, ok, err := queryRow[model.Product](ctx, s.db,
		`SELECT `+productCols+`,
		   COALESCE((SELECT SUM(s.qty) FROM stocks s WHERE s.product_id=p.id),0) AS stock_qty
		 FROM products p LEFT JOIN categories c ON c.id=p.category_id WHERE p.id=?`, id)
	if err != nil {
		return p, err
	}
	if !ok {
		return p, ErrNotFound
	}
	return p, nil
}

// ProductInput 商品入参。
type ProductInput struct {
	Code           string      `json:"code"`
	Name           string      `json:"name"`
	Spec           string      `json:"spec"`
	Unit           string      `json:"unit"`
	CategoryID     int64       `json:"category_id"`
	Barcode        string      `json:"barcode"`
	Brand          string      `json:"brand"`
	PurchasePrice  model.Price `json:"purchase_price"`
	SalePrice      model.Price `json:"sale_price"`
	SafetyStock    model.Qty   `json:"safety_stock"`
	SafetyStockMax model.Qty   `json:"safety_stock_max"`
	Status         int         `json:"status"`
	Remark         string      `json:"remark"`
}

// SaveProduct 新建或更新商品。
func (s *Store) SaveProduct(ctx context.Context, id int64, in ProductInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.TrimSpace(in.Code)
	if in.Name == "" {
		return 0, fmt.Errorf("%w: 商品名称不能为空", ErrBadRequest)
	}
	if in.Unit == "" {
		in.Unit = "个"
	}
	if in.PurchasePrice < 0 || in.SalePrice < 0 {
		return 0, fmt.Errorf("%w: 参考进价和参考售价不能为负数", ErrBadRequest)
	}
	if in.SafetyStock < 0 || in.SafetyStockMax < 0 {
		return 0, fmt.Errorf("%w: 安全库存和库存上限不能为负数", ErrBadRequest)
	}
	if in.CategoryID > 0 {
		var categoryExists int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE id=?`, in.CategoryID).Scan(&categoryExists); err != nil {
			return 0, err
		}
		if categoryExists == 0 {
			return 0, fmt.Errorf("%w: 商品分类不存在", ErrBadRequest)
		}
	}
	now := model.Now()
	if id > 0 {
		if in.Code == "" {
			code, err := existingCode(ctx, s.db, "products", id)
			if err != nil {
				return 0, err
			}
			in.Code = code
		}
		res, err := s.db.ExecContext(ctx,
			`UPDATE products SET code=?, name=?, spec=?, unit=?, category_id=?, barcode=?, brand=?,
			   purchase_price=?, sale_price=?, safety_stock=?, safety_stock_max=?, status=?, remark=?, updated_at=?
			 WHERE id=?`,
			in.Code, in.Name, in.Spec, in.Unit, in.CategoryID, in.Barcode, in.Brand,
			in.PurchasePrice, in.SalePrice, in.SafetyStock, in.SafetyStockMax,
			normStatus(in.Status), in.Remark, now, id)
		if err != nil {
			return 0, wrapErr(err, "update product")
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return 0, ErrNotFound
		}
		return id, nil
	}
	if in.Code == "" {
		code, err := nextSimpleCode(ctx, s.db, "products", "SP", 4)
		if err != nil {
			return 0, err
		}
		in.Code = code
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO products(code, name, spec, unit, category_id, barcode, brand, purchase_price,
		   sale_price, safety_stock, safety_stock_max, status, remark, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.Code, in.Name, in.Spec, in.Unit, in.CategoryID, in.Barcode, in.Brand,
		in.PurchasePrice, in.SalePrice, in.SafetyStock, in.SafetyStockMax,
		normStatus(in.Status), in.Remark, now, now)
	if err != nil {
		return 0, wrapErr(err, "insert product")
	}
	return res.LastInsertId()
}

// DeleteProduct 删除商品。
func (s *Store) DeleteProduct(ctx context.Context, id int64) error {
	checks := []struct {
		sql string
		msg string
	}{
		{`SELECT COUNT(*) FROM stocks WHERE product_id=?`, "该商品仍有库存记录"},
		{`SELECT COUNT(*) FROM purchase_items WHERE product_id=?`, "该商品已被采购单引用"},
		{`SELECT COUNT(*) FROM sales_items WHERE product_id=?`, "该商品已被销售单引用"},
		{`SELECT COUNT(*) FROM transfer_items WHERE product_id=?`, "该商品已被调拨单引用"},
		{`SELECT COUNT(*) FROM stock_take_items WHERE product_id=?`, "该商品已被盘点单引用"},
		{`SELECT COUNT(*) FROM stock_moves WHERE product_id=?`, "该商品存在库存流水"},
	}
	for _, c := range checks {
		var n int
		if err := s.db.QueryRowContext(ctx, c.sql, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%w: %s，建议改为「停用」", ErrInUse, c.msg)
		}
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM products WHERE id=?`, id)
	if err != nil {
		return wrapErr(err, "delete product")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateProductPrices 依据最近一次采购 / 销售更新商品参考价。
func updateProductRefPrice(ctx context.Context, tx *sql.Tx, productID int64, price model.Price, purchase bool) error {
	col := "sale_price"
	if purchase {
		col = "purchase_price"
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE products SET `+col+`=?, updated_at=? WHERE id=?`, price, model.Now(), productID)
	return err
}

// nextSimpleCode 生成形如 SP0001 的编码。
//
// 注意：SQLite 连接池上限为 1，查询必须使用 QueryRowContext 或及时关闭 Rows，
// 否则在持有结果集的情况下继续查询会等待同一个连接，造成阻塞。
func nextSimpleCode(ctx context.Context, db execer, table, prefix string, pad int) (string, error) {
	var next int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0)+1 FROM `+table).Scan(&next); err != nil {
		return "", wrapErr(err, "next code")
	}
	for i := 0; i < 10000; i++ {
		code := prefix + padNum(next+i, pad)
		var exists int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE code=?`, code).Scan(&exists); err != nil {
			return "", wrapErr(err, "check code")
		}
		if exists == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("%w: 无法生成唯一编码，请手工填写编码", ErrBadRequest)
}

// existingCode 读取指定记录的编码，不存在时返回 ErrNotFound。
func existingCode(ctx context.Context, db execer, table string, id int64) (string, error) {
	var code string
	err := db.QueryRowContext(ctx, `SELECT code FROM `+table+` WHERE id=?`, id).Scan(&code)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", wrapErr(err, "load code")
	}
	return code, nil
}

func padNum(n, width int) string {
	s := strconv.Itoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return s
}

// ---------------------------------------------------------------------------
// 往来单位
// ---------------------------------------------------------------------------

const partnerCols = `p.id, p.code, p.name, p.type, p.contact, p.phone, p.address, p.tax_no,
	p.bank_name, p.bank_account, p.credit_limit, p.init_receivable, p.init_payable,
	p.status, p.remark, p.created_at, p.updated_at`

// PartnerQuery 往来单位查询条件。
type PartnerQuery struct {
	Keyword string
	Type    string // customer / supplier / both / ""(全部)
	Status  int
	Page    int
	Size    int
}

// ListPartners 分页查询往来单位。
func (s *Store) ListPartners(ctx context.Context, q PartnerQuery) (model.Page[model.Partner], error) {
	var out model.Page[model.Partner]
	where := []string{"1=1"}
	args := []any{}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(p.code LIKE ? OR p.name LIKE ? OR p.contact LIKE ? OR p.phone LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	switch q.Type {
	case "customer":
		where = append(where, "p.type IN ('customer','both')")
	case "supplier":
		where = append(where, "p.type IN ('supplier','both')")
	case "both":
		where = append(where, "p.type='both'")
	}
	if q.Status == 0 || q.Status == 1 {
		where = append(where, "p.status=?")
		args = append(args, q.Status)
	}
	cond := strings.Join(where, " AND ")
	total, err := countOf(ctx, s.db, `SELECT COUNT(*) FROM partners p WHERE `+cond, args...)
	if err != nil {
		return out, err
	}
	page, size := normPage(q.Page, q.Size)
	args2 := append(append([]any{}, args...), size, (page-1)*size)
	list, err := query[model.Partner](ctx, s.db,
		`SELECT `+partnerCols+`,
		   CASE WHEN p.type IN ('customer','both') THEN p.init_receivable ELSE 0 END
		   + COALESCE((SELECT SUM(o.settle_amount - o.received_amount)
		               FROM sales_orders o WHERE o.customer_id=p.id AND o.status=1),0)
		   - COALESCE((SELECT SUM(pm.amount)
		               FROM payments pm WHERE pm.partner_id=p.id AND pm.type='receipt'
		                 AND pm.status=1 AND pm.order_id=0),0) AS receivable,
		   CASE WHEN p.type IN ('supplier','both') THEN p.init_payable ELSE 0 END
		   + COALESCE((SELECT SUM(o.settle_amount - o.paid_amount)
		               FROM purchase_orders o WHERE o.supplier_id=p.id AND o.status=1),0)
		   - COALESCE((SELECT SUM(pm.amount)
		               FROM payments pm WHERE pm.partner_id=p.id AND pm.type='payment'
		                 AND pm.status=1 AND pm.order_id=0),0) AS payable
		 FROM partners p WHERE `+cond+` ORDER BY p.id DESC LIMIT ? OFFSET ?`, args2...)
	if err != nil {
		return out, err
	}
	return model.Page[model.Partner]{List: list, Total: total, Page: page, Size: size}, nil
}

// AllPartners 查询全部启用往来单位。
func (s *Store) AllPartners(ctx context.Context, typ, keyword string) ([]model.Partner, error) {
	where := []string{"p.status=1"}
	args := []any{}
	switch typ {
	case "customer":
		where = append(where, "p.type IN ('customer','both')")
	case "supplier":
		where = append(where, "p.type IN ('supplier','both')")
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		where = append(where, "(p.code LIKE ? OR p.name LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	return query[model.Partner](ctx, s.db,
		`SELECT `+partnerCols+` FROM partners p WHERE `+strings.Join(where, " AND ")+
			` ORDER BY p.code LIMIT 300`, args...)
}

// GetPartner 按 ID 获取往来单位（含应收应付余额）。
func (s *Store) GetPartner(ctx context.Context, id int64) (model.Partner, error) {
	p, ok, err := queryRow[model.Partner](ctx, s.db,
		`SELECT `+partnerCols+` FROM partners p WHERE p.id=?`, id)
	if err != nil {
		return p, err
	}
	if !ok {
		return p, ErrNotFound
	}
	ar, ap, err := s.PartnerBalance(ctx, id)
	if err != nil {
		return p, err
	}
	p.Receivable = ar
	p.Payable = ap
	return p, nil
}

// PartnerBalance 计算往来单位应收 / 应付余额。
func (s *Store) PartnerBalance(ctx context.Context, id int64) (model.Money, model.Money, error) {
	var initRecv, initPay int64
	var typ string
	if err := s.db.QueryRowContext(ctx,
		`SELECT init_receivable, init_payable, type FROM partners WHERE id=?`, id).
		Scan(&initRecv, &initPay, &typ); err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, ErrNotFound
		}
		return 0, 0, err
	}
	// 与应收应付报表保持一致：纯供应商的期初应收、纯客户的期初应付
	// 不参与余额计算，避免历史脏数据让首页和报表口径不一致。
	if typ == "supplier" {
		initRecv = 0
	}
	if typ == "customer" {
		initPay = 0
	}
	var saleTotal, saleReceived, purTotal, purPaid int64
	// 只有已确认的单据才影响应收应付，草稿和已作废单据不计入。
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(settle_amount),0), COALESCE(SUM(received_amount),0)
		 FROM sales_orders WHERE customer_id=? AND status=1`, id).Scan(&saleTotal, &saleReceived); err != nil {
		return 0, 0, err
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(settle_amount),0), COALESCE(SUM(paid_amount),0)
		 FROM purchase_orders WHERE supplier_id=? AND status=1`, id).Scan(&purTotal, &purPaid); err != nil {
		return 0, 0, err
	}
	// 未关联单据的收付款
	var recvOther, payOther int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM payments
		 WHERE partner_id=? AND type='receipt' AND status=1 AND order_id=0`, id).Scan(&recvOther); err != nil {
		return 0, 0, err
	}
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount),0) FROM payments
		 WHERE partner_id=? AND type='payment' AND status=1 AND order_id=0`, id).Scan(&payOther); err != nil {
		return 0, 0, err
	}
	ar := model.Money(initRecv + saleTotal - saleReceived - recvOther)
	ap := model.Money(initPay + purTotal - purPaid - payOther)
	return ar, ap, nil
}

// PartnerInput 往来单位入参。
type PartnerInput struct {
	Code           string      `json:"code"`
	Name           string      `json:"name"`
	Type           string      `json:"type"`
	Contact        string      `json:"contact"`
	Phone          string      `json:"phone"`
	Address        string      `json:"address"`
	TaxNo          string      `json:"tax_no"`
	BankName       string      `json:"bank_name"`
	BankAccount    string      `json:"bank_account"`
	CreditLimit    model.Money `json:"credit_limit"`
	InitReceivable model.Money `json:"init_receivable"`
	InitPayable    model.Money `json:"init_payable"`
	Status         int         `json:"status"`
	Remark         string      `json:"remark"`
}

// SavePartner 新建或更新往来单位。
func (s *Store) SavePartner(ctx context.Context, id int64, in PartnerInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.TrimSpace(in.Code)
	if in.Name == "" {
		return 0, fmt.Errorf("%w: 单位名称不能为空", ErrBadRequest)
	}
	switch in.Type {
	case "customer", "supplier", "both":
	default:
		in.Type = "customer"
	}
	// 期初余额必须与单位类型匹配。应收应付报表按类型筛选单位，如果
	// 纯供应商带期初应收、纯客户带期初应付，就会出现首页/列表有金额、
	// 对账报表却找不到对应单位的“总数对不上”问题。
	if in.Type == "supplier" && in.InitReceivable != 0 {
		return 0, fmt.Errorf("%w: 纯供应商不能填写期初应收；如同时经营请选择「客户兼供应商」", ErrBadRequest)
	}
	if in.Type == "customer" && in.InitPayable != 0 {
		return 0, fmt.Errorf("%w: 纯客户不能填写期初应付；如同时经营请选择「客户兼供应商」", ErrBadRequest)
	}
	now := model.Now()
	if id > 0 {
		// 往来单位类型决定应收/应付报表是否展示该单位。若已有销售/收款
		// 却改成纯供应商（或反之），欠款会从报表中消失，必须阻止。
		if in.Type == "supplier" {
			var salesRefs, receiptRefs int
			if err := s.db.QueryRowContext(ctx,
				`SELECT
				   (SELECT COUNT(*) FROM sales_orders WHERE customer_id=? AND status<>2),
				   (SELECT COUNT(*) FROM payments WHERE partner_id=? AND type='receipt' AND status<>2)`,
				id, id).Scan(&salesRefs, &receiptRefs); err != nil {
				return 0, err
			}
			if salesRefs > 0 || receiptRefs > 0 {
				return 0, fmt.Errorf("%w: 该单位已有销售或收款记录，不能改为纯供应商；如同时经营请选择「客户兼供应商」", ErrBadRequest)
			}
		}
		if in.Type == "customer" {
			var purchaseRefs, paymentRefs int
			if err := s.db.QueryRowContext(ctx,
				`SELECT
				   (SELECT COUNT(*) FROM purchase_orders WHERE supplier_id=? AND status<>2),
				   (SELECT COUNT(*) FROM payments WHERE partner_id=? AND type='payment' AND status<>2)`,
				id, id).Scan(&purchaseRefs, &paymentRefs); err != nil {
				return 0, err
			}
			if purchaseRefs > 0 || paymentRefs > 0 {
				return 0, fmt.Errorf("%w: 该单位已有采购或付款记录，不能改为纯客户；如同时经营请选择「客户兼供应商」", ErrBadRequest)
			}
		}
		if in.Code == "" {
			code, err := existingCode(ctx, s.db, "partners", id)
			if err != nil {
				return 0, err
			}
			in.Code = code
		}
		res, err := s.db.ExecContext(ctx,
			`UPDATE partners SET code=?, name=?, type=?, contact=?, phone=?, address=?, tax_no=?,
			   bank_name=?, bank_account=?, credit_limit=?, init_receivable=?, init_payable=?,
			   status=?, remark=?, updated_at=? WHERE id=?`,
			in.Code, in.Name, in.Type, in.Contact, in.Phone, in.Address, in.TaxNo,
			in.BankName, in.BankAccount, in.CreditLimit, in.InitReceivable, in.InitPayable,
			normStatus(in.Status), in.Remark, now, id)
		if err != nil {
			return 0, wrapErr(err, "update partner")
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return 0, ErrNotFound
		}
		return id, nil
	}
	if in.Code == "" {
		prefix := "KH"
		if in.Type == "supplier" {
			prefix = "GYS"
		}
		code, err := nextSimpleCode(ctx, s.db, "partners", prefix, 4)
		if err != nil {
			return 0, err
		}
		in.Code = code
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO partners(code, name, type, contact, phone, address, tax_no, bank_name, bank_account,
		   credit_limit, init_receivable, init_payable, status, remark, created_at, updated_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.Code, in.Name, in.Type, in.Contact, in.Phone, in.Address, in.TaxNo,
		in.BankName, in.BankAccount, in.CreditLimit, in.InitReceivable, in.InitPayable,
		normStatus(in.Status), in.Remark, now, now)
	if err != nil {
		return 0, wrapErr(err, "insert partner")
	}
	return res.LastInsertId()
}

// DeletePartner 删除往来单位。
func (s *Store) DeletePartner(ctx context.Context, id int64) error {
	var initRecv, initPay int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT init_receivable, init_payable FROM partners WHERE id=?`, id).
		Scan(&initRecv, &initPay); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	if initRecv != 0 || initPay != 0 {
		return fmt.Errorf("%w: 该单位仍有期初应收/应付余额，请先结清或调整后再删除", ErrInUse)
	}
	checks := []struct {
		sql string
		msg string
	}{
		{`SELECT COUNT(*) FROM purchase_orders WHERE supplier_id=?`, "该单位存在采购单"},
		{`SELECT COUNT(*) FROM sales_orders WHERE customer_id=?`, "该单位存在销售单"},
		{`SELECT COUNT(*) FROM payments WHERE partner_id=?`, "该单位存在收付款记录"},
	}
	for _, c := range checks {
		var n int
		if err := s.db.QueryRowContext(ctx, c.sql, id).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return fmt.Errorf("%w: %s，建议改为「停用」", ErrInUse, c.msg)
		}
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM partners WHERE id=?`, id)
	if err != nil {
		return wrapErr(err, "delete partner")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// 仓库
// ---------------------------------------------------------------------------

// ListWarehouses 查询仓库（含库存统计）。
func (s *Store) ListWarehouses(ctx context.Context, onlyEnabled bool) ([]model.Warehouse, error) {
	cond := "1=1"
	if onlyEnabled {
		cond = "w.status=1"
	}
	return query[model.Warehouse](ctx, s.db,
		`SELECT w.id, w.code, w.name, w.address, w.contact, w.phone, w.is_default, w.sort,
		   w.status, w.remark, w.created_at, w.updated_at,
		   COALESCE((SELECT COUNT(*) FROM stocks s WHERE s.warehouse_id=w.id AND s.qty<>0),0) AS product_kinds,
		   COALESCE((SELECT SUM(s.qty) FROM stocks s WHERE s.warehouse_id=w.id),0) AS total_qty,
		   COALESCE((SELECT SUM(s.amount) FROM stocks s WHERE s.warehouse_id=w.id),0) AS total_amount
		 FROM warehouses w WHERE `+cond+` ORDER BY w.sort, w.id`)
}

// GetWarehouse 按 ID 获取仓库。
func (s *Store) GetWarehouse(ctx context.Context, id int64) (model.Warehouse, error) {
	w, ok, err := queryRow[model.Warehouse](ctx, s.db,
		`SELECT id, code, name, address, contact, phone, is_default, sort, status, remark,
		   created_at, updated_at FROM warehouses WHERE id=?`, id)
	if err != nil {
		return w, err
	}
	if !ok {
		return w, ErrNotFound
	}
	return w, nil
}

// DefaultWarehouse 返回默认仓库。
func (s *Store) DefaultWarehouse(ctx context.Context) (model.Warehouse, error) {
	w, ok, err := queryRow[model.Warehouse](ctx, s.db,
		`SELECT id, code, name, address, contact, phone, is_default, sort, status, remark,
		   created_at, updated_at FROM warehouses WHERE status=1 ORDER BY is_default DESC, sort, id LIMIT 1`)
	if err != nil {
		return w, err
	}
	if !ok {
		return w, ErrNotFound
	}
	return w, nil
}

// WarehouseInput 仓库入参。
type WarehouseInput struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	Contact   string `json:"contact"`
	Phone     string `json:"phone"`
	IsDefault bool   `json:"is_default"`
	Sort      int    `json:"sort"`
	Status    int    `json:"status"`
	Remark    string `json:"remark"`
}

// SaveWarehouse 新建或更新仓库。
func (s *Store) SaveWarehouse(ctx context.Context, id int64, in WarehouseInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.TrimSpace(in.Code)
	if in.Name == "" {
		return 0, fmt.Errorf("%w: 仓库名称不能为空", ErrBadRequest)
	}
	now := model.Now()
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		if in.IsDefault {
			if _, err := tx.ExecContext(ctx, `UPDATE warehouses SET is_default=0`); err != nil {
				return err
			}
		}
		if id > 0 {
			if in.Code == "" {
				code, err := existingCode(ctx, tx, "warehouses", id)
				if err != nil {
					return err
				}
				in.Code = code
			}
			res, err := tx.ExecContext(ctx,
				`UPDATE warehouses SET code=?, name=?, address=?, contact=?, phone=?, is_default=?,
				   sort=?, status=?, remark=?, updated_at=? WHERE id=?`,
				in.Code, in.Name, in.Address, in.Contact, in.Phone, boolInt(in.IsDefault),
				in.Sort, normStatus(in.Status), in.Remark, now, id)
			if err != nil {
				return wrapErr(err, "update warehouse")
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return ErrNotFound
			}
			return nil
		}
		if in.Code == "" {
			code, err := nextSimpleCode(ctx, tx, "warehouses", "WH", 3)
			if err != nil {
				return err
			}
			in.Code = code
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO warehouses(code, name, address, contact, phone, is_default, sort, status, remark, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			in.Code, in.Name, in.Address, in.Contact, in.Phone, boolInt(in.IsDefault),
			in.Sort, normStatus(in.Status), in.Remark, now, now)
		return wrapErr(err, "insert warehouse")
	})
	if err != nil {
		return id, err
	}
	if id == 0 {
		_ = s.db.QueryRowContext(ctx, `SELECT id FROM warehouses WHERE code=?`, in.Code).Scan(&id)
	}
	return id, nil
}

// DeleteWarehouse 删除仓库。
func (s *Store) DeleteWarehouse(ctx context.Context, id int64) error {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stocks WHERE warehouse_id=? AND qty<>0`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w: 该仓库仍有库存，无法删除", ErrInUse)
	}
	checks := []string{
		`SELECT COUNT(*) FROM purchase_orders WHERE warehouse_id=?`,
		`SELECT COUNT(*) FROM sales_orders WHERE warehouse_id=?`,
		`SELECT COUNT(*) FROM stock_takes WHERE warehouse_id=?`,
		`SELECT COUNT(*) FROM stock_moves WHERE warehouse_id=?`,
		`SELECT COUNT(*) FROM transfers WHERE from_warehouse_id=? OR to_warehouse_id=?`,
	}
	for i, q := range checks {
		var c int
		var err error
		if i == 4 {
			err = s.db.QueryRowContext(ctx, q, id, id).Scan(&c)
		} else {
			err = s.db.QueryRowContext(ctx, q, id).Scan(&c)
		}
		if err != nil {
			return err
		}
		if c > 0 {
			return fmt.Errorf("%w: 该仓库已被业务单据引用，建议改为「停用」", ErrInUse)
		}
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=?`, id)
	if err != nil {
		return wrapErr(err, "delete warehouse")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// 资金账户
// ---------------------------------------------------------------------------

// ListAccounts 查询资金账户。
func (s *Store) ListAccounts(ctx context.Context, onlyEnabled bool) ([]model.Account, error) {
	cond := "1=1"
	if onlyEnabled {
		cond = "status=1"
	}
	return query[model.Account](ctx, s.db,
		`SELECT id, code, name, type, balance, is_default, sort, status, remark, created_at, updated_at
		 FROM accounts WHERE `+cond+` ORDER BY sort, id`)
}

// GetAccount 按 ID 取账户。
func (s *Store) GetAccount(ctx context.Context, id int64) (model.Account, error) {
	a, ok, err := queryRow[model.Account](ctx, s.db,
		`SELECT id, code, name, type, balance, is_default, sort, status, remark, created_at, updated_at
		 FROM accounts WHERE id=?`, id)
	if err != nil {
		return a, err
	}
	if !ok {
		return a, ErrNotFound
	}
	return a, nil
}

// AccountInput 账户入参。
type AccountInput struct {
	Code      string      `json:"code"`
	Name      string      `json:"name"`
	Type      string      `json:"type"`
	Balance   model.Money `json:"balance"`
	IsDefault bool        `json:"is_default"`
	Sort      int         `json:"sort"`
	Status    int         `json:"status"`
	Remark    string      `json:"remark"`
}

// SaveAccount 新建或更新账户。
func (s *Store) SaveAccount(ctx context.Context, id int64, in AccountInput) (int64, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Code = strings.TrimSpace(in.Code)
	if in.Name == "" {
		return 0, fmt.Errorf("%w: 账户名称不能为空", ErrBadRequest)
	}
	switch in.Type {
	case "cash", "bank", "alipay", "wechat", "other":
	default:
		in.Type = "cash"
	}
	now := model.Now()
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		if in.IsDefault {
			if _, err := tx.ExecContext(ctx, `UPDATE accounts SET is_default=0`); err != nil {
				return err
			}
		}
		if id > 0 {
			if in.Code == "" {
				code, err := existingCode(ctx, tx, "accounts", id)
				if err != nil {
					return err
				}
				in.Code = code
			}
			// 余额不允许通过编辑账户直接覆盖，必须走 AdjustAccount 生成调账记录。
			res, err := tx.ExecContext(ctx,
				`UPDATE accounts SET code=?, name=?, type=?, is_default=?, sort=?,
				   status=?, remark=?, updated_at=? WHERE id=?`,
				in.Code, in.Name, in.Type, boolInt(in.IsDefault), in.Sort,
				normStatus(in.Status), in.Remark, now, id)
			if err != nil {
				return wrapErr(err, "update account")
			}
			if n, _ := res.RowsAffected(); n == 0 {
				return ErrNotFound
			}
			return nil
		}
		if in.Code == "" {
			code, err := nextSimpleCode(ctx, tx, "accounts", "ACC", 3)
			if err != nil {
				return err
			}
			in.Code = code
		}
		_, err := tx.ExecContext(ctx,
			`INSERT INTO accounts(code, name, type, balance, is_default, sort, status, remark, created_at, updated_at)
			 VALUES(?,?,?,?,?,?,?,?,?,?)`,
			in.Code, in.Name, in.Type, in.Balance, boolInt(in.IsDefault), in.Sort,
			normStatus(in.Status), in.Remark, now, now)
		return wrapErr(err, "insert account")
	})
	if err != nil {
		return id, err
	}
	if id == 0 {
		_ = s.db.QueryRowContext(ctx, `SELECT id FROM accounts WHERE code=?`, in.Code).Scan(&id)
	}
	return id, nil
}

// AdjustAccount 调整账户余额，并写入一条独立调账记录。
// 账户余额不能在编辑账户时直接覆盖，所有余额变化都必须经过这里。
func (s *Store) AdjustAccount(ctx context.Context, id int64, balance model.Money, reason string, user OpUser) (model.AccountAdjustment, error) {
	var adj model.AccountAdjustment
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return adj, fmt.Errorf("%w: 请填写调账原因", ErrBadRequest)
	}
	err := s.Tx(ctx, func(tx *sql.Tx) error {
		var name string
		var before int64
		if err := tx.QueryRowContext(ctx,
			`SELECT name, balance FROM accounts WHERE id=?`, id).Scan(&name, &before); err != nil {
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		if int64(balance) == before {
			return fmt.Errorf("%w: 调整后余额与当前余额相同", ErrBadRequest)
		}
		now := model.Now()
		if _, err := tx.ExecContext(ctx,
			`UPDATE accounts SET balance=?, updated_at=? WHERE id=?`, balance, now, id); err != nil {
			return wrapErr(err, "adjust account balance")
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO account_adjustments(account_id, account_name, before_balance, after_balance,
			   amount, reason, created_by, created_by_name, created_at)
			 VALUES(?,?,?,?,?,?,?,?,?)`,
			id, name, before, int64(balance), int64(balance)-before, reason,
			user.ID, user.Name, now)
		if err != nil {
			return wrapErr(err, "insert account adjustment")
		}
		adjID, _ := res.LastInsertId()
		adj = model.AccountAdjustment{
			ID:            adjID,
			AccountID:     id,
			AccountName:   name,
			BeforeBalance: model.Money(before),
			AfterBalance:  balance,
			Amount:        model.Money(int64(balance) - before),
			Reason:        reason,
			CreatedBy:     user.ID,
			CreatedByName: user.Name,
			CreatedAt:     now,
		}
		return nil
	})
	return adj, err
}

// ListAccountAdjustments 查询调账记录；accountID 为 0 时返回全部。
func (s *Store) ListAccountAdjustments(ctx context.Context, accountID int64, limit int) ([]model.AccountAdjustment, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	where := "1=1"
	args := []any{}
	if accountID > 0 {
		where = "a.account_id=?"
		args = append(args, accountID)
	}
	args = append(args, limit)
	return query[model.AccountAdjustment](ctx, s.db,
		`SELECT a.id, a.account_id, a.account_name, a.before_balance, a.after_balance,
		   a.amount, a.reason, a.created_by, a.created_by_name, a.created_at
		 FROM account_adjustments a WHERE `+where+` ORDER BY a.id DESC LIMIT ?`, args...)
}

// DeleteAccount 删除账户。
func (s *Store) DeleteAccount(ctx context.Context, id int64) error {
	var balance int64
	if err := s.db.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE id=?`, id).Scan(&balance); err != nil {
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		return err
	}
	if balance != 0 {
		return fmt.Errorf("%w: 该账户余额不为 0，请先转出或调账后再删除", ErrInUse)
	}
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM payments WHERE account_id=?`, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w: 该账户存在收付款记录，建议改为「停用」", ErrInUse)
	}
	var adjustments int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_adjustments WHERE account_id=?`, id).Scan(&adjustments); err != nil {
		return err
	}
	if adjustments > 0 {
		return fmt.Errorf("%w: 该账户存在调账记录，建议改为「停用」", ErrInUse)
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id)
	if err != nil {
		return wrapErr(err, "delete account")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
