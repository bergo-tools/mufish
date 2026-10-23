package store

import (
	"context"
	"errors"
	"testing"

	"mufish/internal/model"
)

func TestUserRoleMustExist(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if _, err := st.CreateUser(ctx, UserInput{
		Username: "req014_bad_role", Password: "abc123456", Name: "错误角色", RoleID: 999999, Status: 1,
	}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("不存在的角色应返回 ErrBadRequest，实际 %v", err)
	}
	var count int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username=?`, "req014_bad_role").Scan(&count); err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("非法角色失败后不应写入用户，实际 %d 条", count)
	}

	// role_id=0 仍表示“未分配角色”，属于允许的显式状态。
	uid, err := st.CreateUser(ctx, UserInput{
		Username: "req014_no_role", Password: "abc123456", Name: "未分配", RoleID: 0, Status: 1,
	})
	if err != nil {
		t.Fatalf("role_id=0 应允许创建: %v", err)
	}

	if err := st.UpdateUser(ctx, uid, UserInput{
		Username: "req014_no_role", Name: "未分配", RoleID: 888888, Status: 1,
	}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("更新为不存在的角色应返回 ErrBadRequest，实际 %v", err)
	}
	u, err := st.GetUser(ctx, uid)
	if err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if u.RoleID != 0 {
		t.Fatalf("非法角色更新失败后角色不应改变，实际 %d", u.RoleID)
	}
}

func TestNormalizeDocDateRejectsInvalidDate(t *testing.T) {
	if got, err := normalizeDocDate("2026-01-02"); err != nil || got != "2026-01-02" {
		t.Fatalf("合法日期应通过: %q / %v", got, err)
	}
	if got, err := normalizeDocDate("2026-01-02-extra"); err != nil || got != "2026-01-02" {
		t.Fatalf("带后缀的合法日期应按前 10 位规范化: %q / %v", got, err)
	}
	for _, bad := range []string{"2026-13-01", "2026-02-30", "abc"} {
		if _, err := normalizeDocDate(bad); !errors.Is(err, ErrBadRequest) {
			t.Fatalf("非法日期 %q 应返回 ErrBadRequest，实际 %v", bad, err)
		}
	}
}

func addProduct(t *testing.T, st *Store, code, name string, purchase, sale model.Price) int64 {
	t.Helper()
	id, err := st.SaveProduct(context.Background(), 0, ProductInput{
		Code: code, Name: name, Unit: "个",
		PurchasePrice: purchase, SalePrice: sale, Status: 1,
	})
	if err != nil {
		t.Fatalf("创建商品 %s 失败: %v", code, err)
	}
	return id
}

func rowsByCode(rows []model.RankRow) map[string]model.RankRow {
	m := make(map[string]model.RankRow, len(rows))
	for _, r := range rows {
		m[r.Code] = r
	}
	return m
}

// 销售排行必须按单据结算金额分摊单据级优惠/其他费用，否则会和首页
// 销售/毛利、客户排行使用两个口径。
func TestSalesRankAllocatesHeaderAdjustment(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)
	pid2 := addProduct(t, st, "T-P002", "测试商品B", model.PriceFrom(20), model.PriceFrom(50))

	p, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{
			{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)},
			{ProductID: pid2, Qty: model.QtyFrom(10), Price: model.PriceFrom(20)},
		},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}

	s, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		DiscountAmount: model.YuanFrom(10), OtherFee: model.YuanFrom(5),
		Items: []OrderItemInput{
			{ProductID: pid, Qty: model.QtyFrom(2), Price: model.PriceFrom(30)},
			{ProductID: pid2, Qty: model.QtyFrom(1), Price: model.PriceFrom(50)},
		},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, s, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	rows, err := st.SalesRank(ctx, RankQuery{Limit: 10})
	if err != nil {
		t.Fatalf("查询销售排行失败: %v", err)
	}
	byCode := rowsByCode(rows)
	a, ok := byCode["T-P001"]
	if !ok {
		t.Fatalf("销售排行缺少商品 A: %+v", rows)
	}
	b, ok := byCode["T-P002"]
	if !ok {
		t.Fatalf("销售排行缺少商品 B: %+v", rows)
	}
	// 结算金额 110 - 10 + 5 = 105 元，按明细金额 60:50 分摊。
	if a.Amount != model.YuanFrom(57.27) || b.Amount != model.YuanFrom(47.73) {
		t.Fatalf("优惠/费用分摊不正确：A=%s B=%s", a.Amount, b.Amount)
	}
	if a.Cost != model.YuanFrom(20) || b.Cost != model.YuanFrom(20) {
		t.Fatalf("销售成本不正确：A=%s B=%s", a.Cost, b.Cost)
	}
	if a.Amount+b.Amount != model.YuanFrom(105) {
		t.Fatalf("商品销售额合计应为 105 元，实际 %s", a.Amount+b.Amount)
	}

	d, err := st.Dashboard(ctx)
	if err != nil {
		t.Fatalf("查询看板失败: %v", err)
	}
	if d.MonthSales != a.Amount+b.Amount {
		t.Fatalf("排行销售额合计 %s 应等于看板销售额 %s", a.Amount+b.Amount, d.MonthSales)
	}
	if d.MonthProfit != (a.Amount+b.Amount)-(a.Cost+b.Cost) {
		t.Fatalf("排行毛利合计与看板毛利不一致：%s / %s", (a.Amount+b.Amount)-(a.Cost+b.Cost), d.MonthProfit)
	}

	cr, err := st.CustomerRank(ctx, RankQuery{Limit: 10})
	if err != nil || len(cr) != 1 {
		t.Fatalf("客户排行查询失败或行数异常: %+v / %v", cr, err)
	}
	if cr[0].Amount != a.Amount+b.Amount {
		t.Fatalf("客户排行金额 %s 应等于商品排行合计 %s", cr[0].Amount, a.Amount+b.Amount)
	}
}

// 采购排行同样要按结算金额分摊优惠/费用，不能只看明细金额。
func TestPurchaseRankAllocatesHeaderAdjustment(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)
	pid2 := addProduct(t, st, "T-P002", "测试商品B", model.PriceFrom(20), model.PriceFrom(50))

	p, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		DiscountAmount: model.YuanFrom(5), OtherFee: model.YuanFrom(2),
		Items: []OrderItemInput{
			{ProductID: pid, Qty: model.QtyFrom(2), Price: model.PriceFrom(10)},
			{ProductID: pid2, Qty: model.QtyFrom(3), Price: model.PriceFrom(20)},
		},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}

	rows, err := st.PurchaseRank(ctx, RankQuery{Limit: 10})
	if err != nil {
		t.Fatalf("查询采购排行失败: %v", err)
	}
	byCode := rowsByCode(rows)
	a, ok := byCode["T-P001"]
	if !ok {
		t.Fatalf("采购排行缺少商品 A: %+v", rows)
	}
	b, ok := byCode["T-P002"]
	if !ok {
		t.Fatalf("采购排行缺少商品 B: %+v", rows)
	}
	// 明细 20 + 60 = 80，优惠 5、其他费用 2，结算 77，按 20:60 分摊。
	if a.Amount != model.YuanFrom(19.25) || b.Amount != model.YuanFrom(57.75) {
		t.Fatalf("采购优惠/费用分摊不正确：A=%s B=%s", a.Amount, b.Amount)
	}
	if a.Amount+b.Amount != model.YuanFrom(77) {
		t.Fatalf("商品采购额合计应为 77 元，实际 %s", a.Amount+b.Amount)
	}
	d, err := st.Dashboard(ctx)
	if err != nil {
		t.Fatalf("查询看板失败: %v", err)
	}
	if d.MonthPurchase != a.Amount+b.Amount {
		t.Fatalf("排行采购额合计 %s 应等于看板采购额 %s", a.Amount+b.Amount, d.MonthPurchase)
	}

	sr, err := st.SupplierRank(ctx, RankQuery{Limit: 10})
	if err != nil || len(sr) != 1 {
		t.Fatalf("供应商排行查询失败或行数异常: %+v / %v", sr, err)
	}
	if sr[0].Amount != a.Amount+b.Amount {
		t.Fatalf("供应商排行金额 %s 应等于商品排行合计 %s", sr[0].Amount, a.Amount+b.Amount)
	}
}

// 主数据改名后，排行必须按 ID 聚合为一行并展示当前名称。
func TestRankGroupingUsesCurrentMasterData(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	p, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第一次采购失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p, user); err != nil {
		t.Fatalf("确认第一次采购失败: %v", err)
	}
	s1, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第一次销售失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, s1, user); err != nil {
		t.Fatalf("确认第一次销售失败: %v", err)
	}

	// 直接改基础资料名称，模拟用户在两次业务之间改名。
	if _, err := st.db.ExecContext(ctx, `UPDATE products SET name=? WHERE id=?`, "改名后商品", pid); err != nil {
		t.Fatalf("改名商品失败: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE partners SET name=? WHERE id=?`, "改名后客户", customer); err != nil {
		t.Fatalf("改名客户失败: %v", err)
	}
	if _, err := st.db.ExecContext(ctx, `UPDATE partners SET name=? WHERE id=?`, "改名后供应商", supplier); err != nil {
		t.Fatalf("改名供应商失败: %v", err)
	}

	p2, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(2), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第二次采购失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p2, user); err != nil {
		t.Fatalf("确认第二次采购失败: %v", err)
	}
	s2, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第二次销售失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, s2, user); err != nil {
		t.Fatalf("确认第二次销售失败: %v", err)
	}

	salesRank, err := st.SalesRank(ctx, RankQuery{Limit: 10})
	if err != nil {
		t.Fatalf("查询销售排行失败: %v", err)
	}
	if len(salesRank) != 1 {
		t.Fatalf("商品改名后销售排行应只有 1 行，实际 %d: %+v", len(salesRank), salesRank)
	}
	if salesRank[0].Name != "改名后商品" {
		t.Fatalf("销售排行应展示当前商品名，实际 %q", salesRank[0].Name)
	}

	purchaseRank, err := st.PurchaseRank(ctx, RankQuery{Limit: 10})
	if err != nil {
		t.Fatalf("查询采购排行失败: %v", err)
	}
	if len(purchaseRank) != 1 {
		t.Fatalf("商品改名后采购排行应只有 1 行，实际 %d: %+v", len(purchaseRank), purchaseRank)
	}
	if purchaseRank[0].Name != "改名后商品" {
		t.Fatalf("采购排行应展示当前商品名，实际 %q", purchaseRank[0].Name)
	}

	customerRank, err := st.CustomerRank(ctx, RankQuery{Limit: 10})
	if err != nil {
		t.Fatalf("查询客户排行失败: %v", err)
	}
	if len(customerRank) != 1 {
		t.Fatalf("客户改名后排行应只有 1 行，实际 %d: %+v", len(customerRank), customerRank)
	}
	if customerRank[0].Name != "改名后客户" {
		t.Fatalf("客户排行应展示当前客户名，实际 %q", customerRank[0].Name)
	}

	supplierRank, err := st.SupplierRank(ctx, RankQuery{Limit: 10})
	if err != nil {
		t.Fatalf("查询供应商排行失败: %v", err)
	}
	if len(supplierRank) != 1 {
		t.Fatalf("供应商改名后排行应只有 1 行，实际 %d: %+v", len(supplierRank), supplierRank)
	}
	if supplierRank[0].Name != "改名后供应商" {
		t.Fatalf("供应商排行应展示当前供应商名，实际 %q", supplierRank[0].Name)
	}
}
