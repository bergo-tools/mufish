package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"mufish/internal/model"
)

// newTestStore 建立一个全新的测试数据库。
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestSeededAdminCanAuthenticate(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	u, perms, err := st.Authenticate(ctx, "admin", DefaultAdminPassword)
	if err != nil {
		t.Fatalf("内置管理员登录失败: %v", err)
	}
	if !u.IsSuper {
		t.Fatalf("内置管理员应拥有 is_super 标记")
	}
	if !perms.Has("product:create") || !perms.Has("anything:at:all") {
		t.Fatalf("超级管理员应通过任意权限校验")
	}

	if _, _, err := st.Authenticate(ctx, "admin", "wrong-password"); err == nil {
		t.Fatalf("错误口令应登录失败")
	}
}

func TestPasswordHashing(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	u, err := st.GetUserByName(ctx, "admin")
	if err != nil {
		t.Fatalf("查询用户失败: %v", err)
	}
	if u.Password == DefaultAdminPassword {
		t.Fatalf("口令不应以明文保存")
	}
	if err := st.UpdatePassword(ctx, u.ID, DefaultAdminPassword, "newpass123"); err != nil {
		t.Fatalf("修改口令失败: %v", err)
	}
	if _, _, err := st.Authenticate(ctx, "admin", "newpass123"); err != nil {
		t.Fatalf("新口令应可登录: %v", err)
	}
	if _, _, err := st.Authenticate(ctx, "admin", DefaultAdminPassword); err == nil {
		t.Fatalf("旧口令应失效")
	}
}

// 建立一个干净的业务环境（不使用演示数据）。
func bootstrap(t *testing.T, st *Store) (productID, supplierID, customerID, warehouseID int64) {
	t.Helper()
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}

	productID, err := st.SaveProduct(ctx, 0, ProductInput{
		Code: "T-P001", Name: "测试商品A", Spec: "标准", Unit: "个",
		PurchasePrice: model.PriceFrom(10), SalePrice: model.PriceFrom(20),
		SafetyStock: model.QtyFrom(5), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	supplierID, err = st.SavePartner(ctx, 0, PartnerInput{
		Code: "T-S001", Name: "测试供应商", Type: "supplier", Status: 1,
	})
	if err != nil {
		t.Fatalf("创建供应商失败: %v", err)
	}
	customerID, err = st.SavePartner(ctx, 0, PartnerInput{
		Code: "T-C001", Name: "测试客户", Type: "customer", Status: 1,
	})
	if err != nil {
		t.Fatalf("创建客户失败: %v", err)
	}
	wh, err := st.DefaultWarehouse(ctx)
	if err != nil {
		t.Fatalf("获取默认仓库失败: %v", err)
	}
	warehouseID = wh.ID
	_ = user
	return
}

func stockQty(t *testing.T, st *Store, warehouseID, productID int64) (model.Qty, model.Price) {
	t.Helper()
	ctx := context.Background()
	stocks, err := st.ProductStockByWarehouse(ctx, productID)
	if err != nil {
		t.Fatalf("查询库存失败: %v", err)
	}
	for _, s := range stocks {
		if s.WarehouseID == warehouseID {
			return s.Qty, s.CostPrice
		}
	}
	return 0, 0
}

func TestPurchaseThenSalesCosting(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	// --- 采购 10 个 @10 元 ---
	p1, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	order, err := st.GetPurchase(ctx, p1)
	if err != nil {
		t.Fatalf("读取采购单失败: %v", err)
	}
	if order.Status != model.DocDraft {
		t.Fatalf("新建采购单应为草稿状态，实际 %d", order.Status)
	}
	if order.SettleAmount != model.YuanFrom(100) {
		t.Fatalf("采购结算金额期望 100 元，实际 %s", order.SettleAmount)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 0 {
		t.Fatalf("未确认入库前不应有库存，实际 %s", q.String())
	}

	// 确认入库
	if err := st.ConfirmPurchase(ctx, p1, user); err != nil {
		t.Fatalf("入库确认失败: %v", err)
	}
	q, price := stockQty(t, st, wid, pid)
	if q != 10 {
		t.Fatalf("入库后库存期望 10，实际 %s", q.String())
	}
	if price != model.PriceFrom(10) {
		t.Fatalf("入库后成本期望 10 元，实际 %s", price.String())
	}

	// 重复确认应报错
	if err := st.ConfirmPurchase(ctx, p1, user); err == nil {
		t.Fatalf("重复入库确认应报错")
	}

	// --- 再采购 10 个 @20 元，验证移动加权 ---
	p2, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第二张采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p2, user); err != nil {
		t.Fatalf("第二张采购单入库失败: %v", err)
	}
	q, price = stockQty(t, st, wid, pid)
	if q != 20 {
		t.Fatalf("二次入库后期望 20，实际 %s", q.String())
	}
	if price != model.PriceFrom(15) { // (10*10 + 10*20) / 20 = 15
		t.Fatalf("移动加权成本期望 15 元，实际 %s", price.String())
	}

	// --- 销售 4 个 @30 元 ---
	s1, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(4), Price: model.PriceFrom(30)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, s1, user); err != nil {
		t.Fatalf("出库确认失败: %v", err)
	}
	sale, err := st.GetSales(ctx, s1)
	if err != nil {
		t.Fatalf("读取销售单失败: %v", err)
	}
	if sale.SettleAmount != model.YuanFrom(120) {
		t.Fatalf("销售额期望 120 元，实际 %s", sale.SettleAmount)
	}
	if sale.CostAmount != model.YuanFrom(60) { // 4 * 15
		t.Fatalf("销售成本期望 60 元，实际 %s", sale.CostAmount)
	}
	q, price = stockQty(t, st, wid, pid)
	if q != 16 {
		t.Fatalf("出库后期望 16，实际 %s", q.String())
	}
	if price != model.PriceFrom(15) {
		t.Fatalf("出库后成本单价应保持 15 元，实际 %s", price.String())
	}

	// --- 库存不足应被拒绝 ---
	s2, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(999), Price: model.PriceFrom(30)}},
	}, user)
	if err != nil {
		t.Fatalf("创建超额销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, s2, user); err == nil {
		t.Fatalf("库存不足时应拒绝出库")
	} else if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("库存不足应返回参数类错误，实际 %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 16 {
		t.Fatalf("失败的出库不应改变库存，实际 %s", q.String())
	}

	// --- 作废销售单应回冲库存 ---
	if err := st.VoidSales(ctx, s1, user); err != nil {
		t.Fatalf("作废销售单失败: %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 20 {
		t.Fatalf("作废销售单后期望恢复到 20，实际 %s", q.String())
	}

	// --- 作废采购单应冲减库存 ---
	if err := st.VoidPurchase(ctx, p2, user); err != nil {
		t.Fatalf("作废采购单失败: %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 10 {
		t.Fatalf("作废第二张采购单后期望 10，实际 %s", q.String())
	}
}

func TestStockTakeAndTransfer(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)

	p, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(100), Price: model.PriceFrom(8)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p, user); err != nil {
		t.Fatalf("入库失败: %v", err)
	}

	// --- 盘点：账面 100，实盘 96 ---
	takeID, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(96)}},
	}, user)
	if err != nil {
		t.Fatalf("创建盘点单失败: %v", err)
	}
	take, err := st.GetStockTake(ctx, takeID)
	if err != nil {
		t.Fatalf("读取盘点单失败: %v", err)
	}
	if take.DiffQty != model.QtyFrom(-4) {
		t.Fatalf("盘点差异期望 -4，实际 %s", take.DiffQty.String())
	}
	if take.DiffAmount != model.YuanFrom(-32) {
		t.Fatalf("盘点差异金额期望 -32 元，实际 %s", take.DiffAmount)
	}
	if err := st.FinishStockTake(ctx, takeID, user); err != nil {
		t.Fatalf("完成盘点失败: %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 96 {
		t.Fatalf("盘点后期望 96，实际 %s", q.String())
	}

	// 作废盘点应回滚
	if err := st.VoidStockTake(ctx, takeID, user); err != nil {
		t.Fatalf("作废盘点失败: %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 100 {
		t.Fatalf("作废盘点后期望 100，实际 %s", q.String())
	}

	// --- 调拨到新仓库 ---
	w2, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "T-WH2", Name: "测试二仓", Status: 1})
	if err != nil {
		t.Fatalf("创建仓库失败: %v", err)
	}
	tr, err := st.SaveTransfer(ctx, 0, TransferInput{
		FromWarehouseID: wid, ToWarehouseID: w2, TransferDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(30)}},
	}, user)
	if err != nil {
		t.Fatalf("创建调拨单失败: %v", err)
	}
	if err := st.ConfirmTransfer(ctx, tr, user); err != nil {
		t.Fatalf("确认调拨失败: %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 70 {
		t.Fatalf("调出仓期望 70，实际 %s", q.String())
	}
	q2, price2 := stockQty(t, st, w2, pid)
	if q2 != 30 {
		t.Fatalf("调入仓期望 30，实际 %s", q2.String())
	}
	if price2 != model.PriceFrom(8) {
		t.Fatalf("调拨应结转原成本 8 元，实际 %s", price2.String())
	}

	// 作废调拨
	if err := st.ConfirmTransfer(ctx, tr, user); err == nil {
		t.Fatalf("重复确认调拨应报错")
	}
	if err := st.VoidTransfer(ctx, tr, user); err != nil {
		t.Fatalf("作废调拨失败: %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 100 {
		t.Fatalf("作废调拨后调出仓期望 100，实际 %s", q.String())
	}
	if q2, _ := stockQty(t, st, w2, pid); q2 != 0 {
		t.Fatalf("作废调拨后调入仓期望 0，实际 %s", q2.String())
	}
}

func TestPaymentsAndArAp(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	p, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p, user); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	s, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(5), Price: model.PriceFrom(30)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, s, user); err != nil {
		t.Fatalf("出库失败: %v", err)
	}

	ar, ap, err := st.PartnerBalance(ctx, customer)
	if err != nil {
		t.Fatalf("查询客户余额失败: %v", err)
	}
	if ar != model.YuanFrom(150) {
		t.Fatalf("客户应收期望 150 元，实际 %s", ar)
	}
	if ap != 0 {
		t.Fatalf("客户应付期望 0，实际 %s", ap)
	}
	_, apSup, err := st.PartnerBalance(ctx, supplier)
	if err != nil {
		t.Fatalf("查询供应商余额失败: %v", err)
	}
	if apSup != model.YuanFrom(100) {
		t.Fatalf("供应商应付期望 100 元，实际 %s", apSup)
	}

	accounts, err := st.ListAccounts(ctx, true)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("应存在默认资金账户: %v", err)
	}
	acc := accounts[0]

	// 收款 100
	pay, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: acc.ID,
		PayDate: model.Today(), Amount: model.YuanFrom(100), Method: "bank",
		OrderType: "sales", OrderID: s,
	}, user)
	if err != nil {
		t.Fatalf("创建收款单失败: %v", err)
	}
	if err := st.ConfirmPayment(ctx, pay, user); err != nil {
		t.Fatalf("确认收款失败: %v", err)
	}
	if ar, _, _ := st.PartnerBalance(ctx, customer); ar != model.YuanFrom(50) {
		t.Fatalf("收款后应收期望 50 元，实际 %s", ar)
	}
	if a, _ := st.GetAccount(ctx, acc.ID); a.Balance != model.YuanFrom(100) {
		t.Fatalf("收款后账户余额期望 100 元，实际 %s", a.Balance)
	}

	// 超额收款应被拒绝
	over, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: acc.ID,
		PayDate: model.Today(), Amount: model.YuanFrom(999), Method: "bank",
		OrderType: "sales", OrderID: s,
	}, user)
	if err != nil {
		t.Fatalf("创建超额收款单失败: %v", err)
	}
	if err := st.ConfirmPayment(ctx, over, user); err == nil {
		t.Fatalf("超过未收金额的收款应被拒绝")
	}

	// 作废收款应回滚
	if err := st.VoidPayment(ctx, pay, user); err != nil {
		t.Fatalf("作废收款失败: %v", err)
	}
	if ar, _, _ := st.PartnerBalance(ctx, customer); ar != model.YuanFrom(150) {
		t.Fatalf("作废收款后应收期望 150 元，实际 %s", ar)
	}
	if a, _ := st.GetAccount(ctx, acc.ID); a.Balance != 0 {
		t.Fatalf("作废收款后账户余额期望 0，实际 %s", a.Balance)
	}

	// 已确认的销售单不可删除
	if err := st.DeleteSales(ctx, s); err == nil {
		t.Fatalf("已确认的销售单不应允许删除")
	}
}

func TestRbacAndReports(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	roles, err := st.ListRoles(ctx)
	if err != nil {
		t.Fatalf("查询角色失败: %v", err)
	}
	if len(roles) < 5 {
		t.Fatalf("应内置多个角色，实际 %d", len(roles))
	}
	var viewer model.Role
	for _, r := range roles {
		if r.Code == "viewer" {
			viewer = r
		}
	}
	if viewer.ID == 0 {
		t.Fatalf("应内置 viewer 角色")
	}
	viewerPerms := model.NewPermSet(viewer.Perms)
	if !viewerPerms.Has("product:view") {
		t.Fatalf("viewer 应拥有 product:view")
	}
	if viewerPerms.Has("product:delete") {
		t.Fatalf("viewer 不应拥有 product:delete")
	}

	// 自定义角色
	rid, err := st.CreateRole(ctx, RoleInput{Code: "t_role", Name: "测试角色", Perms: []string{"stock:view", "不存在的权限"}})
	if err != nil {
		t.Fatalf("创建角色失败: %v", err)
	}
	role, err := st.GetRole(ctx, rid)
	if err != nil {
		t.Fatalf("读取角色失败: %v", err)
	}
	if len(role.Perms) != 1 || role.Perms[0] != "stock:view" {
		t.Fatalf("非法权限应被过滤，实际 %v", role.Perms)
	}
	uid, err := st.CreateUser(ctx, UserInput{Username: "t_user", Password: "abc123456", Name: "测试用户", RoleID: rid, Status: 1})
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	u, err := st.GetUser(ctx, uid)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	perms, err := st.UserPerms(ctx, u)
	if err != nil {
		t.Fatalf("计算权限失败: %v", err)
	}
	if perms.Has("product:delete") || !perms.Has("stock:view") {
		t.Fatalf("用户权限计算不正确: %v", perms.List())
	}
	// 角色被引用时不可删除
	if err := st.DeleteRole(ctx, rid); err == nil {
		t.Fatalf("仍被用户使用的角色不应允许删除")
	}
	if err := st.DeleteUser(ctx, uid); err != nil {
		t.Fatalf("删除用户失败: %v", err)
	}
	if err := st.DeleteRole(ctx, rid); err != nil {
		t.Fatalf("删除空闲角色失败: %v", err)
	}

	// 内置角色不可删除
	for _, r := range roles {
		if r.Code == "admin" {
			if err := st.DeleteRole(ctx, r.ID); err == nil {
				t.Fatalf("内置角色不应允许删除")
			}
		}
	}

	// 看板与报表可正常执行
	if _, err := st.Dashboard(ctx); err != nil {
		t.Fatalf("看板查询失败: %v", err)
	}
	if _, err := st.Trend(ctx, 14); err != nil {
		t.Fatalf("走势查询失败: %v", err)
	}
	if _, err := st.InoutSummary(ctx, InoutQuery{From: "2000-01-01", To: "2999-12-31"}); err != nil {
		t.Fatalf("进销存汇总失败: %v", err)
	}
	if _, err := st.ArApReport(ctx, ArApQuery{Type: "receivable"}); err != nil {
		t.Fatalf("应收报表失败: %v", err)
	}
	if _, err := st.SalesRank(ctx, RankQuery{Limit: 10}); err != nil {
		t.Fatalf("销售排行失败: %v", err)
	}
}

func TestBusinessDocumentLifecycle(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)

	// 空明细应被拒绝
	if _, err := st.SavePurchase(ctx, 0, PurchaseInput{SupplierID: supplier, WarehouseID: wid}, user); err == nil {
		t.Fatalf("没有明细的采购单应被拒绝")
	}
	// 数量为 0 应被拒绝
	if _, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid,
		Items: []OrderItemInput{{ProductID: pid, Qty: 0, Price: model.PriceFrom(1)}},
	}, user); err == nil {
		t.Fatalf("数量为 0 的明细应被拒绝")
	}

	id, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		DiscountAmount: model.YuanFrom(10), OtherFee: model.YuanFrom(5),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	o, _ := st.GetPurchase(ctx, id)
	// 100 - 10 + 5 = 95
	if o.SettleAmount != model.YuanFrom(95) {
		t.Fatalf("结算金额期望 95 元，实际 %s", o.SettleAmount)
	}
	if o.TotalQty != 10 {
		t.Fatalf("合计数量期望 10，实际 %s", o.TotalQty.String())
	}

	// 草稿可修改
	if _, err := st.SavePurchase(ctx, id, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(20), Price: model.PriceFrom(10)}},
	}, user); err != nil {
		t.Fatalf("修改草稿采购单失败: %v", err)
	}
	o, _ = st.GetPurchase(ctx, id)
	if o.TotalQty != 20 || len(o.Items) != 1 {
		t.Fatalf("修改后应只有一行 20 个，实际 %s / %d 行", o.TotalQty.String(), len(o.Items))
	}

	if err := st.ConfirmPurchase(ctx, id, user); err != nil {
		t.Fatalf("入库失败: %v", err)
	}
	// 已确认不可修改
	if _, err := st.SavePurchase(ctx, id, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid,
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(1)}},
	}, user); err == nil {
		t.Fatalf("已确认的采购单不应允许修改")
	}
	// 单据编号唯一且格式正确
	if len(o.OrderNo) < 10 || o.OrderNo[:2] != "CG" {
		t.Fatalf("采购单编号格式异常: %s", o.OrderNo)
	}
}
