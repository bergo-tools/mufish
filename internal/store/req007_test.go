package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"mufish/internal/model"
)

func stockState(t *testing.T, st *Store, warehouseID, productID int64) (float64, int64, int64) {
	t.Helper()
	var qty float64
	var amount, price int64
	err := st.db.QueryRow(
		`SELECT qty, amount, cost_price FROM stocks WHERE warehouse_id=? AND product_id=?`,
		warehouseID, productID,
	).Scan(&qty, &amount, &price)
	if err != nil {
		t.Fatalf("查询库存状态失败: %v", err)
	}
	return qty, amount, price
}

func buy(t *testing.T, st *Store, supplier, warehouse, product int64, qty float64, price float64) int64 {
	t.Helper()
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	id, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: warehouse, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: product, Qty: model.QtyFrom(qty), Price: model.PriceFrom(price)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, id, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	return id
}

// P0-1：作废采购单必须按原入库金额冲减，而不是按当前移动平均成本。
func TestP0VoidPurchaseUsesOriginalAmount(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)

	first := buy(t, st, supplier, wid, pid, 10, 10)
	buy(t, st, supplier, wid, pid, 10, 20)

	qty, amount, price := stockState(t, st, wid, pid)
	if qty != 20 || amount != int64(model.YuanFrom(300)) || price != int64(model.PriceFrom(15)) {
		t.Fatalf("作废前库存应为 20 个 / 300 元 / 均价 15 元，实际 %.4f / %d / %d", qty, amount, price)
	}

	if err := st.VoidPurchase(ctx, first, user); err != nil {
		t.Fatalf("作废采购单失败: %v", err)
	}
	qty, amount, price = stockState(t, st, wid, pid)
	if qty != 10 {
		t.Fatalf("作废后数量应为 10，实际 %.4f", qty)
	}
	if amount != int64(model.YuanFrom(200)) {
		t.Fatalf("作废后结存金额应为 200 元，实际 %s（仍按当前均价冲减会得到 150 元）", model.Money(amount))
	}
	if price != int64(model.PriceFrom(20)) {
		t.Fatalf("作废后结存单价应为 20 元，实际 %s", model.Price(price))
	}
}

// P0-1：调拨作废同样按原调拨金额恢复，避免调入仓后续入库造成成本失真。
func TestP0VoidTransferUsesOriginalAmount(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)
	buy(t, st, supplier, wid, pid, 10, 10)

	toID, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "T-W002", Name: "调拨仓", Status: 1})
	if err != nil {
		t.Fatalf("创建调拨仓失败: %v", err)
	}
	transferID, err := st.SaveTransfer(ctx, 0, TransferInput{
		FromWarehouseID: wid, ToWarehouseID: toID, TransferDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(2), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建调拨单失败: %v", err)
	}
	if err := st.ConfirmTransfer(ctx, transferID, user); err != nil {
		t.Fatalf("确认调拨单失败: %v", err)
	}
	buy(t, st, supplier, toID, pid, 2, 20)

	qty, amount, price := stockState(t, st, toID, pid)
	if qty != 4 || amount != int64(model.YuanFrom(60)) || price != int64(model.PriceFrom(15)) {
		t.Fatalf("调拨后调入仓应为 4 个 / 60 元 / 均价 15 元，实际 %.4f / %d / %d", qty, amount, price)
	}

	if err := st.VoidTransfer(ctx, transferID, user); err != nil {
		t.Fatalf("作废调拨单失败: %v", err)
	}
	qty, amount, price = stockState(t, st, toID, pid)
	if qty != 2 || amount != int64(model.YuanFrom(40)) || price != int64(model.PriceFrom(20)) {
		t.Fatalf("调拨作废后调入仓应为 2 个 / 40 元 / 单价 20 元，实际 %.4f / %d / %d", qty, amount, price)
	}
	qty, amount, _ = stockState(t, st, wid, pid)
	if qty != 10 || amount != int64(model.YuanFrom(100)) {
		t.Fatalf("调拨作废后调出仓应为 10 个 / 100 元，实际 %.4f / %d", qty, amount)
	}
}

// P0-2：管理员在编辑用户时修改口令，也必须让该用户所有旧会话失效。
func TestP0UpdateUserPasswordInvalidatesSessions(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	u, err := st.GetUserByName(ctx, "admin")
	if err != nil {
		t.Fatalf("查询管理员失败: %v", err)
	}

	token, _, err := st.CreateSession(ctx, u.ID, "127.0.0.1", "test", time.Hour)
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if _, ok := st.SessionUser(ctx, token); !ok {
		t.Fatalf("新会话应有效")
	}

	if err := st.UpdateUser(ctx, u.ID, UserInput{
		Name: u.Name, Phone: u.Phone, Email: u.Email, RoleID: u.RoleID,
		Status: 1, Password: "newpass123",
	}); err != nil {
		t.Fatalf("编辑用户口令失败: %v", err)
	}
	if _, ok := st.SessionUser(ctx, token); ok {
		t.Fatalf("修改口令后旧会话必须失效")
	}
	if _, _, err := st.Authenticate(ctx, "admin", "newpass123"); err != nil {
		t.Fatalf("新口令应可登录: %v", err)
	}

	// 不修改口令时，只是改姓名/状态不应踢掉会话。
	token2, _, err := st.CreateSession(ctx, u.ID, "127.0.0.1", "test", time.Hour)
	if err != nil {
		t.Fatalf("创建第二个会话失败: %v", err)
	}
	if err := st.UpdateUser(ctx, u.ID, UserInput{
		Name: "管理员", Phone: u.Phone, Email: u.Email, RoleID: u.RoleID, Status: 1,
	}); err != nil {
		t.Fatalf("编辑用户资料失败: %v", err)
	}
	if _, ok := st.SessionUser(ctx, token2); !ok {
		t.Fatalf("未修改口令时不应删除会话")
	}
}

// P1-1：销售优惠金额不能超过明细金额，失败时不能写入单据。
func TestP1SalesDiscountCannotExceedItems(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, _, customer, wid := bootstrap(t, st)

	_, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		DiscountAmount: model.YuanFrom(11),
		Items:          []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("优惠超过明细金额应返回参数错误，实际 %v", err)
	}
	page, err := st.ListSales(ctx, SalesQuery{Status: -1, Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("查询销售单失败: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("保存失败后不应留下销售单，实际 %d 条", page.Total)
	}

	// 优惠等于明细金额（0 元结算）仍应允许。
	id, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		DiscountAmount: model.YuanFrom(10),
		Items:          []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("优惠等于明细金额应允许: %v", err)
	}
	doc, err := st.GetSales(ctx, id)
	if err != nil {
		t.Fatalf("查询销售单失败: %v", err)
	}
	if doc.SettleAmount != 0 {
		t.Fatalf("结算金额应为 0，实际 %s", doc.SettleAmount)
	}
}

// P1-3：往来冲抵不需要资金账户；非冲抵方式仍必须选择账户。
func TestP1OffsetPaymentDoesNotRequireAccount(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	_, _, customer, _ := bootstrap(t, st)

	id, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: 0,
		PayDate: model.Today(), Amount: model.YuanFrom(50), Method: "offset",
	}, user)
	if err != nil {
		t.Fatalf("冲抵收款不应要求资金账户: %v", err)
	}
	if err := st.ConfirmPayment(ctx, id, user); err != nil {
		t.Fatalf("确认冲抵收款失败: %v", err)
	}
	p, err := st.GetPayment(ctx, id)
	if err != nil {
		t.Fatalf("查询收付款失败: %v", err)
	}
	if p.AccountID != 0 || p.AccountName != "" {
		t.Fatalf("冲抵单据不应关联资金账户，实际 account_id=%d name=%q", p.AccountID, p.AccountName)
	}

	if _, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: 0,
		PayDate: model.Today(), Amount: model.YuanFrom(10), Method: "bank",
	}, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("非冲抵方式没有资金账户应报错，实际 %v", err)
	}
}

// P1-4：资金流水关键字过滤要真正作用到查询。
func TestP1CashFlowKeyword(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	_, _, customer, _ := bootstrap(t, st)
	accounts, err := st.ListAccounts(ctx, true)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("查询账户失败: %v", err)
	}
	id, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: accounts[0].ID,
		PayDate: model.Today(), Amount: model.YuanFrom(30), Method: "bank", Remark: "尾款",
	}, user)
	if err != nil {
		t.Fatalf("创建收款失败: %v", err)
	}
	if err := st.ConfirmPayment(ctx, id, user); err != nil {
		t.Fatalf("确认收款失败: %v", err)
	}
	p, _ := st.GetPayment(ctx, id)

	cases := []struct {
		keyword string
		want    int
	}{
		{"测试客户", 1},
		{p.PaymentNo, 1},
		{"尾款", 1},
		{"不存在的关键字", 0},
	}
	for _, tc := range cases {
		list, err := st.CashFlow(ctx, "", "", tc.keyword, 0, 100)
		if err != nil {
			t.Fatalf("查询资金流水 %q 失败: %v", tc.keyword, err)
		}
		if len(list) != tc.want {
			t.Fatalf("关键字 %q 期望 %d 条，实际 %d 条", tc.keyword, tc.want, len(list))
		}
	}
}
// P0-1：销售作废时也按原出库成本回冲，后续入库不会改变应恢复的金额。
func TestP0VoidSalesRestoresOriginalAmount(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)
	buy(t, st, supplier, wid, pid, 10, 10)

	saleID, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(2), Price: model.PriceFrom(30)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, saleID, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}
	buy(t, st, supplier, wid, pid, 2, 20)

	qty, amount, _ := stockState(t, st, wid, pid)
	if qty != 10 || amount != int64(model.YuanFrom(120)) {
		t.Fatalf("作废前库存应为 10 个 / 120 元，实际 %.4f / %d", qty, amount)
	}
	if err := st.VoidSales(ctx, saleID, user); err != nil {
		t.Fatalf("作废销售单失败: %v", err)
	}
	qty, amount, price := stockState(t, st, wid, pid)
	if qty != 12 || amount != int64(model.YuanFrom(140)) {
		t.Fatalf("作废销售后库存应为 12 个 / 140 元，实际 %.4f / %d", qty, amount)
	}
	if price != int64(model.PriceFrom(11.6667)) {
		t.Fatalf("作废销售后结存单价应为 11.6667 元，实际 %s", model.Price(price))
	}
}
