package store

import (
	"context"
	"strconv"
	"strings"
	"time"

	"mufish/internal/model"
)

// ---------------------------------------------------------------------------
// 首页看板
// ---------------------------------------------------------------------------

// Dashboard 汇总首页数据。
func (s *Store) Dashboard(ctx context.Context) (model.DashboardSummary, error) {
	var d model.DashboardSummary
	today := model.Today()
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local).Format(model.DateLayout)
	nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.Local).Format(model.DateLayout)

	scalar := func(sqlStr string, args ...any) (int64, error) {
		var v int64
		err := s.db.QueryRowContext(ctx, sqlStr, args...).Scan(&v)
		return v, err
	}

	var err error
	if d.TodaySales, err = money1(scalar(`SELECT COALESCE(SUM(settle_amount),0) FROM sales_orders WHERE status=1 AND order_date=?`, today)); err != nil {
		return d, err
	}
	if d.TodayPurchase, err = money1(scalar(`SELECT COALESCE(SUM(settle_amount),0) FROM purchase_orders WHERE status=1 AND order_date=?`, today)); err != nil {
		return d, err
	}
	if d.MonthSales, err = money1(scalar(`SELECT COALESCE(SUM(settle_amount),0) FROM sales_orders WHERE status=1 AND order_date>=? AND order_date<?`, monthStart, nextMonth)); err != nil {
		return d, err
	}
	if d.MonthPurchase, err = money1(scalar(`SELECT COALESCE(SUM(settle_amount),0) FROM purchase_orders WHERE status=1 AND order_date>=? AND order_date<?`, monthStart, nextMonth)); err != nil {
		return d, err
	}
	if d.MonthProfit, err = money1(scalar(
		`SELECT COALESCE(SUM(settle_amount - cost_amount),0) FROM sales_orders WHERE status=1 AND order_date>=? AND order_date<?`, monthStart, nextMonth)); err != nil {
		return d, err
	}

	// 应收 = 期初应收 + 已出库未收 - 未关联单据的收款
	recv, err := scalar(`SELECT COALESCE(SUM(settle_amount - received_amount),0) FROM sales_orders WHERE status=1`)
	if err != nil {
		return d, err
	}
	initRecv, err := scalar(`SELECT COALESCE(SUM(init_receivable),0) FROM partners WHERE type IN ('customer','both')`)
	if err != nil {
		return d, err
	}
	otherRecv, err := scalar(`SELECT COALESCE(SUM(amount),0) FROM payments WHERE type='receipt' AND status=1 AND order_id=0`)
	if err != nil {
		return d, err
	}
	d.Receivable = model.Money(recv + initRecv - otherRecv)

	pay, err := scalar(`SELECT COALESCE(SUM(settle_amount - paid_amount),0) FROM purchase_orders WHERE status=1`)
	if err != nil {
		return d, err
	}
	initPay, err := scalar(`SELECT COALESCE(SUM(init_payable),0) FROM partners WHERE type IN ('supplier','both')`)
	if err != nil {
		return d, err
	}
	otherPay, err := scalar(`SELECT COALESCE(SUM(amount),0) FROM payments WHERE type='payment' AND status=1 AND order_id=0`)
	if err != nil {
		return d, err
	}
	d.Payable = model.Money(pay + initPay - otherPay)

	if d.StockAmount, err = money1(scalar(`SELECT COALESCE(SUM(amount),0) FROM stocks`)); err != nil {
		return d, err
	}
	kinds, err := scalar(`SELECT COUNT(DISTINCT product_id) FROM stocks WHERE qty<>0`)
	if err != nil {
		return d, err
	}
	d.StockKinds = int(kinds)

	low, err := scalar(`SELECT COUNT(*) FROM products p
		WHERE p.status=1 AND p.safety_stock>0
		  AND COALESCE((SELECT SUM(s.qty) FROM stocks s WHERE s.product_id=p.id),0) < p.safety_stock`)
	if err != nil {
		return d, err
	}
	d.LowStockCount = int(low)

	pc, err := scalar(`SELECT COUNT(*) FROM products WHERE status=1`)
	if err != nil {
		return d, err
	}
	d.ProductCount = int(pc)
	nc, err := scalar(`SELECT COUNT(*) FROM partners WHERE status=1`)
	if err != nil {
		return d, err
	}
	d.PartnerCount = int(nc)
	// 资金余额统计全部账户，与资金账户页保持一致；停用只代表不能新增
	// 收付款，账户里的余额仍然是企业资金。
	if d.AccountTotal, err = money1(scalar(`SELECT COALESCE(SUM(balance),0) FROM accounts`)); err != nil {
		return d, err
	}
	return d, nil
}

func money1(v int64, err error) (model.Money, error) {
	if err != nil {
		return 0, err
	}
	return model.Money(v), nil
}

// Trend 最近 n 天的销售 / 采购 / 毛利走势。
func (s *Store) Trend(ctx context.Context, days int) ([]model.TrendPoint, error) {
	if days <= 0 || days > 90 {
		days = 14
	}
	start := time.Now().AddDate(0, 0, -(days - 1)).Format(model.DateLayout)
	type row struct {
		Day      string      `db:"day"`
		Sales    model.Money `db:"sales"`
		Purchase model.Money `db:"purchase"`
	}
	rows, err := query[row](ctx, s.db,
		`SELECT d.day,
		   COALESCE((SELECT SUM(settle_amount) FROM sales_orders WHERE status=1 AND order_date=d.day),0) AS sales,
		   COALESCE((SELECT SUM(settle_amount) FROM purchase_orders WHERE status=1 AND order_date=d.day),0) AS purchase
		 FROM (SELECT order_date AS day FROM sales_orders WHERE order_date>=?
		       UNION SELECT order_date FROM purchase_orders WHERE order_date>=?) d
		 ORDER BY d.day`, start, start)
	if err != nil {
		return nil, err
	}
	byDay := make(map[string]model.TrendPoint, len(rows))
	for _, r := range rows {
		byDay[r.Day] = model.TrendPoint{Date: r.Day, Sales: r.Sales, Purchase: r.Purchase}
	}
	profitRows, err := query[row](ctx, s.db,
		`SELECT order_date AS day, COALESCE(SUM(settle_amount - cost_amount),0) AS sales
		 FROM sales_orders WHERE status=1 AND order_date>=? GROUP BY order_date`, start)
	if err != nil {
		return nil, err
	}
	profitByDay := make(map[string]model.Money, len(profitRows))
	for _, r := range profitRows {
		profitByDay[r.Day] = r.Sales
	}
	out := make([]model.TrendPoint, 0, days)
	for i := 0; i < days; i++ {
		day := time.Now().AddDate(0, 0, -(days - 1 - i)).Format(model.DateLayout)
		p := byDay[day]
		p.Date = day
		p.Profit = profitByDay[day]
		out = append(out, p)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 进销存汇总
// ---------------------------------------------------------------------------

// InoutQuery 进销存汇总查询条件。
type InoutQuery struct {
	From        string
	To          string
	WarehouseID int64
	CategoryID  int64
	Keyword     string
}

// InoutSummary 进销存汇总表。
//
// 期初 = 查询开始日之前的所有流水；本期 = 开始日到结束日（含当天）的流水；
// 期末 = 期初 + 本期入库 - 本期出库。期末不能直接取当前库存，否则历史区间会失真。
func (s *Store) InoutSummary(ctx context.Context, q InoutQuery) ([]model.InoutRow, error) {
	from := q.From
	to := q.To
	if from == "" {
		from = "0000-01-01"
	}
	if to == "" {
		to = "9999-12-31"
	}
	args := []any{}
	cond := []string{"1=1"}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		cond = append(cond, "(p.code LIKE ? OR p.name LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	if q.CategoryID > 0 {
		catCond, cargs := categoryTreeCond("p.category_id", q.CategoryID)
		cond = append(cond, catCond)
		args = append(args, cargs...)
	}
	whCond := ""
	if q.WarehouseID > 0 {
		whCond = " AND m.warehouse_id=" + strconv.FormatInt(q.WarehouseID, 10)
	}
	where := strings.Join(cond, " AND ")

	sqlStr := `
	SELECT p.code AS code, p.name AS name, p.spec AS spec, p.unit AS unit,
	  COALESCE((SELECT SUM(m.qty) FROM stock_moves m
	            WHERE m.product_id=p.id AND m.biz_date < ?` + whCond + `),0) AS init_qty,
	  COALESCE((SELECT SUM(CASE WHEN m.direction=1 THEN m.amount ELSE -m.amount END)
	            FROM stock_moves m
	            WHERE m.product_id=p.id AND m.biz_date < ?` + whCond + `),0) AS init_amount,
	  COALESCE((SELECT SUM(m.qty) FROM stock_moves m
	            WHERE m.product_id=p.id AND m.direction=1 AND m.biz_date >= ? AND m.biz_date <= ?` + whCond + `),0) AS in_qty,
	  COALESCE((SELECT SUM(m.amount) FROM stock_moves m
	            WHERE m.product_id=p.id AND m.direction=1 AND m.biz_date >= ? AND m.biz_date <= ?` + whCond + `),0) AS in_amount,
	  COALESCE((SELECT SUM(-m.qty) FROM stock_moves m
	            WHERE m.product_id=p.id AND m.direction=-1 AND m.biz_date >= ? AND m.biz_date <= ?` + whCond + `),0) AS out_qty,
	  COALESCE((SELECT SUM(m.amount) FROM stock_moves m
	            WHERE m.product_id=p.id AND m.direction=-1 AND m.biz_date >= ? AND m.biz_date <= ?` + whCond + `),0) AS out_amount
	FROM products p
	WHERE ` + where + `
	ORDER BY p.code`

	qargs := []any{from, from, from, to, from, to, from, to, from, to}
	qargs = append(qargs, args...)

	rows, err := query[model.InoutRow](ctx, s.db, sqlStr, qargs...)
	if err != nil {
		return nil, err
	}
	out := make([]model.InoutRow, 0, len(rows))
	for _, r := range rows {
		r.EndQty = r.InitQty + r.InQty - r.OutQty
		r.EndAmount = r.InitAmount + r.InAmount - r.OutAmount
		if r.InitQty == 0 && r.InQty == 0 && r.OutQty == 0 {
			continue
		}
		r.Key = r.Code
		out = append(out, r)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 销售 / 采购排行
// ---------------------------------------------------------------------------

// RankQuery 排行查询条件。
type RankQuery struct {
	From    string
	To      string
	Limit   int
	GroupBy string // product / customer / supplier
}

// SalesRank 销售排行（按商品）。
func (s *Store) SalesRank(ctx context.Context, q RankQuery) ([]model.RankRow, error) {
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 50
	}
	from, to := dateRange(q.From, q.To)
	return query[model.RankRow](ctx, s.db,
		`WITH s AS (
		   SELECT i.product_code AS code, i.product_name AS name, i.spec AS spec, i.unit AS unit,
		     SUM(i.qty) AS qty, SUM(i.amount) AS amount,
		     SUM(i.cost_price * i.qty) / 100.0 AS cost,
		     COUNT(DISTINCT i.order_id) AS doc_count
		   FROM sales_items i JOIN sales_orders o ON o.id = i.order_id
		   WHERE o.status=1 AND o.order_date >= ? AND o.order_date <= ?
		   GROUP BY i.product_id, i.product_code, i.product_name, i.spec, i.unit
		 )
		 SELECT code, name, spec, unit, qty, amount,
		   CAST(ROUND(cost) AS INTEGER) AS cost,
		   CAST(ROUND(amount - cost) AS INTEGER) AS profit,
		   CASE WHEN amount > 0 THEN ROUND((amount - cost) * 100.0 / amount, 4) ELSE 0 END AS rate,
		   doc_count
		 FROM s ORDER BY amount DESC LIMIT ?`, from, to, q.Limit)
}

// CustomerRank 客户销售排行。
func (s *Store) CustomerRank(ctx context.Context, q RankQuery) ([]model.RankRow, error) {
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 50
	}
	from, to := dateRange(q.From, q.To)
	return query[model.RankRow](ctx, s.db,
		`SELECT o.customer_name AS name, '' AS code, '' AS spec, '' AS unit,
		   SUM(o.total_qty) AS qty, SUM(o.settle_amount) AS amount, SUM(o.cost_amount) AS cost,
		   SUM(o.settle_amount - o.cost_amount) AS profit,
		   CASE WHEN SUM(o.settle_amount) > 0
		     THEN SUM(o.settle_amount - o.cost_amount) * 100.0 / SUM(o.settle_amount) ELSE 0 END AS rate,
		   COUNT(*) AS doc_count
		 FROM sales_orders o
		 WHERE o.status=1 AND o.order_date >= ? AND o.order_date <= ?
		 GROUP BY o.customer_id, o.customer_name
		 ORDER BY amount DESC LIMIT ?`, from, to, q.Limit)
}

// SupplierRank 供应商采购排行。
func (s *Store) SupplierRank(ctx context.Context, q RankQuery) ([]model.RankRow, error) {
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 50
	}
	from, to := dateRange(q.From, q.To)
	return query[model.RankRow](ctx, s.db,
		`SELECT o.supplier_name AS name, '' AS code, '' AS spec, '' AS unit,
		   SUM(o.total_qty) AS qty, SUM(o.settle_amount) AS amount, 0 AS cost, 0 AS profit, 0 AS rate,
		   COUNT(*) AS doc_count
		 FROM purchase_orders o
		 WHERE o.status=1 AND o.order_date >= ? AND o.order_date <= ?
		 GROUP BY o.supplier_id, o.supplier_name
		 ORDER BY amount DESC LIMIT ?`, from, to, q.Limit)
}

// PurchaseRank 采购排行（按商品）。
func (s *Store) PurchaseRank(ctx context.Context, q RankQuery) ([]model.RankRow, error) {
	if q.Limit <= 0 || q.Limit > 500 {
		q.Limit = 50
	}
	from, to := dateRange(q.From, q.To)
	return query[model.RankRow](ctx, s.db,
		`SELECT i.product_code AS code, i.product_name AS name, i.spec AS spec, i.unit AS unit,
		   SUM(i.qty) AS qty, SUM(i.amount) AS amount, 0 AS cost, 0 AS profit, 0 AS rate,
		   COUNT(DISTINCT i.order_id) AS doc_count
		 FROM purchase_items i JOIN purchase_orders o ON o.id = i.order_id
		 WHERE o.status=1 AND o.order_date >= ? AND o.order_date <= ?
		 GROUP BY i.product_id, i.product_code, i.product_name, i.spec, i.unit
		 ORDER BY amount DESC LIMIT ?`, from, to, q.Limit)
}

// ProfitByProduct 商品毛利表（销售明细口径）。
func (s *Store) ProfitByProduct(ctx context.Context, q RankQuery) ([]model.RankRow, error) {
	return s.SalesRank(ctx, q)
}

// ---------------------------------------------------------------------------
// 应收 / 应付
// ---------------------------------------------------------------------------

// ArApQuery 应收应付查询条件。
type ArApQuery struct {
	Type    string // receivable / payable
	Keyword string
}

// ArApReport 应收 / 应付余额表。
func (s *Store) ArApReport(ctx context.Context, q ArApQuery) ([]model.ArApRow, error) {
	args := []any{}
	// 应收应付是余额对账表，停用单位只要仍有欠款就必须保留；
	// 否则会和首页应收/应付合计（不区分启用状态）对不上。
	cond := "1=1"
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		cond += " AND (p.code LIKE ? OR p.name LIKE ?)"
		like := "%" + kw + "%"
		args = append(args, like, like)
	}
	var sqlStr string
	if q.Type == "payable" {
		cond += " AND p.type IN ('supplier','both')"
		sqlStr = `
		SELECT p.id AS partner_id, p.name AS partner_name, p.contact AS contact, p.phone AS phone,
		  p.init_payable AS init_amount,
		  COALESCE((SELECT SUM(o.settle_amount) FROM purchase_orders o WHERE o.supplier_id=p.id AND o.status=1),0) AS doc_amount,
		  COALESCE((SELECT SUM(o.paid_amount) FROM purchase_orders o WHERE o.supplier_id=p.id AND o.status=1),0)
		    + COALESCE((SELECT SUM(pm.amount) FROM payments pm WHERE pm.partner_id=p.id AND pm.type='payment' AND pm.status=1 AND pm.order_id=0),0) AS paid_amount,
		  0 AS balance
		FROM partners p WHERE ` + cond + ` ORDER BY p.code`
	} else {
		cond += " AND p.type IN ('customer','both')"
		sqlStr = `
		SELECT p.id AS partner_id, p.name AS partner_name, p.contact AS contact, p.phone AS phone,
		  p.init_receivable AS init_amount,
		  COALESCE((SELECT SUM(o.settle_amount) FROM sales_orders o WHERE o.customer_id=p.id AND o.status=1),0) AS doc_amount,
		  COALESCE((SELECT SUM(o.received_amount) FROM sales_orders o WHERE o.customer_id=p.id AND o.status=1),0)
		    + COALESCE((SELECT SUM(pm.amount) FROM payments pm WHERE pm.partner_id=p.id AND pm.type='receipt' AND pm.status=1 AND pm.order_id=0),0) AS paid_amount,
		  0 AS balance
		FROM partners p WHERE ` + cond + ` ORDER BY p.code`
	}
	rows, err := query[model.ArApRow](ctx, s.db, sqlStr, args...)
	if err != nil {
		return nil, err
	}
	out := make([]model.ArApRow, 0, len(rows))
	for _, r := range rows {
		r.Balance = model.Money(int64(r.InitAmount) + int64(r.DocAmount) - int64(r.PaidAmount))
		out = append(out, r)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// 明细账
// ---------------------------------------------------------------------------

// DocDetailQuery 单据明细查询条件。
type DocDetailQuery struct {
	From        string
	To          string
	WarehouseID int64
	PartnerID   int64
	ProductID   int64
	Keyword     string
	Limit       int
}

// PurchaseDetail 采购明细表。
func (s *Store) PurchaseDetail(ctx context.Context, q DocDetailQuery) ([]model.OrderItem, error) {
	from, to := dateRange(q.From, q.To)
	where := []string{"o.status=1", "o.order_date>=?", "o.order_date<=?"}
	args := []any{from, to}
	if q.WarehouseID > 0 {
		where = append(where, "o.warehouse_id=?")
		args = append(args, q.WarehouseID)
	}
	if q.PartnerID > 0 {
		where = append(where, "o.supplier_id=?")
		args = append(args, q.PartnerID)
	}
	if q.ProductID > 0 {
		where = append(where, "i.product_id=?")
		args = append(args, q.ProductID)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(o.order_no LIKE ? OR i.product_code LIKE ? OR i.product_name LIKE ? OR o.supplier_name LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	limit := q.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	args = append(args, limit)
	return query[model.OrderItem](ctx, s.db,
		`SELECT i.id, i.order_id AS doc_id, o.order_no AS doc_no, o.order_date AS doc_date,
		   i.product_id, i.product_code, i.product_name, i.spec, i.unit,
		   i.qty, i.price, i.amount, i.remark, 0 AS sort
		 FROM purchase_items i JOIN purchase_orders o ON o.id=i.order_id
		 WHERE `+strings.Join(where, " AND ")+` ORDER BY o.order_date DESC, o.id DESC LIMIT ?`, args...)
}

// SalesDetail 销售明细表。
func (s *Store) SalesDetail(ctx context.Context, q DocDetailQuery) ([]model.OrderItem, error) {
	from, to := dateRange(q.From, q.To)
	where := []string{"o.status=1", "o.order_date>=?", "o.order_date<=?"}
	args := []any{from, to}
	if q.WarehouseID > 0 {
		where = append(where, "o.warehouse_id=?")
		args = append(args, q.WarehouseID)
	}
	if q.PartnerID > 0 {
		where = append(where, "o.customer_id=?")
		args = append(args, q.PartnerID)
	}
	if q.ProductID > 0 {
		where = append(where, "i.product_id=?")
		args = append(args, q.ProductID)
	}
	if kw := strings.TrimSpace(q.Keyword); kw != "" {
		where = append(where, "(o.order_no LIKE ? OR i.product_code LIKE ? OR i.product_name LIKE ? OR o.customer_name LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	limit := q.Limit
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	args = append(args, limit)
	return query[model.OrderItem](ctx, s.db,
		`SELECT i.id, i.order_id AS doc_id, o.order_no AS doc_no, o.order_date AS doc_date,
		   i.product_id, i.product_code, i.product_name, i.spec, i.unit,
		   i.qty, i.price, i.amount, i.remark, 0 AS sort
		 FROM sales_items i JOIN sales_orders o ON o.id=i.order_id
		 WHERE `+strings.Join(where, " AND ")+` ORDER BY o.order_date DESC, o.id DESC LIMIT ?`, args...)
}

// ---------------------------------------------------------------------------
// 资金流水
// ---------------------------------------------------------------------------

// CashFlow 资金流水表。
func (s *Store) CashFlow(ctx context.Context, from, to, keyword string, accountID int64, limit int) ([]model.Payment, error) {
	from, to = dateRange(from, to)
	where := []string{"status=1", "pay_date>=?", "pay_date<=?", "method<>'offset'"}
	args := []any{from, to}
	if kw := strings.TrimSpace(keyword); kw != "" {
		where = append(where, "(payment_no LIKE ? OR partner_name LIKE ? OR order_no LIKE ? OR remark LIKE ?)")
		like := "%" + kw + "%"
		args = append(args, like, like, like, like)
	}
	if accountID > 0 {
		where = append(where, "account_id=?")
		args = append(args, accountID)
	}
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	args = append(args, limit)
	return query[model.Payment](ctx, s.db,
		`SELECT `+paymentCols+` FROM payments o WHERE `+strings.Join(where, " AND ")+
			` ORDER BY pay_date DESC, id DESC LIMIT ?`, args...)
}

// StockSummary 库存汇总（按仓库 / 分类）。
func (s *Store) StockSummary(ctx context.Context, warehouseID, categoryID int64) (map[string]any, error) {
	type row struct {
		Kinds  int         `db:"kinds"`
		Qty    model.Qty   `db:"qty"`
		Amount model.Money `db:"amount"`
	}
	where := []string{"1=1"}
	args := []any{}
	if warehouseID > 0 {
		where = append(where, "st.warehouse_id=?")
		args = append(args, warehouseID)
	}
	if categoryID > 0 {
		cond, cargs := categoryTreeCond("p.category_id", categoryID)
		where = append(where, cond)
		args = append(args, cargs...)
	}
	r, _, err := queryRow[row](ctx, s.db,
		`SELECT COUNT(DISTINCT CASE WHEN st.qty<>0 THEN st.product_id END) AS kinds,
		   COALESCE(SUM(st.qty),0) AS qty, COALESCE(SUM(st.amount),0) AS amount
		 FROM stocks st JOIN products p ON p.id=st.product_id WHERE `+strings.Join(where, " AND "), args...)
	if err != nil {
		return nil, err
	}
	return map[string]any{"kinds": r.Kinds, "qty": r.Qty, "amount": r.Amount}, nil
}

func dateRange(from, to string) (string, string) {
	if from == "" {
		from = "0000-01-01"
	}
	if to == "" {
		to = "9999-12-31"
	}
	return from, to
}
