package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"mufish/internal/model"
)

// P0-1：往来单位列表必须返回与实际一致的应收/应付余额。
func TestP0PartnerListIncludesBalances(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	// 客户期初应收 100 元。
	if _, err := st.SavePartner(ctx, customer, PartnerInput{
		Code: "T-C001", Name: "测试客户", Type: "customer",
		InitReceivable: model.YuanFrom(100), Status: 1,
	}); err != nil {
		t.Fatalf("设置客户期初应收失败: %v", err)
	}

	// 采购 10 个，单价 10 元；销售 2 个，单价 100 元。
	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	so, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(2), Price: model.PriceFrom(100)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, so, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	// 未关联销售单的收款 50 元。
	accounts, err := st.ListAccounts(ctx, true)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("查询资金账户失败: %v", err)
	}
	payID, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: accounts[0].ID,
		PayDate: model.Today(), Amount: model.YuanFrom(50), Method: "bank",
	}, user)
	if err != nil {
		t.Fatalf("创建收款单失败: %v", err)
	}
	if err := st.ConfirmPayment(ctx, payID, user); err != nil {
		t.Fatalf("确认收款单失败: %v", err)
	}

	page, err := st.ListPartners(ctx, PartnerQuery{Status: -1, Page: 1, Size: 50})
	if err != nil {
		t.Fatalf("查询往来单位失败: %v", err)
	}
	var gotCustomer, gotSupplier model.Partner
	for _, p := range page.List {
		switch p.ID {
		case customer:
			gotCustomer = p
		case supplier:
			gotSupplier = p
		}
	}
	if gotCustomer.ID == 0 || gotSupplier.ID == 0 {
		t.Fatalf("列表中未找到测试往来单位")
	}

	wantCustomerAR, _, err := st.PartnerBalance(ctx, customer)
	if err != nil {
		t.Fatalf("查询客户余额失败: %v", err)
	}
	_, wantSupplierAP, err := st.PartnerBalance(ctx, supplier)
	if err != nil {
		t.Fatalf("查询供应商余额失败: %v", err)
	}
	if gotCustomer.Receivable != wantCustomerAR || gotSupplier.Payable != wantSupplierAP {
		t.Fatalf("列表余额与详情不一致：客户列表 %s / 详情 %s，供应商列表 %s / 详情 %s",
			gotCustomer.Receivable, wantCustomerAR, gotSupplier.Payable, wantSupplierAP)
	}
	if wantCustomerAR != model.YuanFrom(250) { // 100 期初 + 200 销售 - 50 收款
		t.Fatalf("客户应收期望 250 元，实际 %s", wantCustomerAR)
	}
	if wantSupplierAP != model.YuanFrom(100) {
		t.Fatalf("供应商应付期望 100 元，实际 %s", wantSupplierAP)
	}
}

// P0-2：往来冲抵只核销应收应付，不产生资金账户变动或资金流水。
func TestP0OffsetPaymentDoesNotTouchCash(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	_, _, customer, _ := bootstrap(t, st)
	accounts, err := st.ListAccounts(ctx, true)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("查询资金账户失败: %v", err)
	}
	acc := accounts[0]

	payID, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: acc.ID,
		PayDate: model.Today(), Amount: model.YuanFrom(50), Method: "offset",
	}, user)
	if err != nil {
		t.Fatalf("创建冲抵收款失败: %v", err)
	}
	if err := st.ConfirmPayment(ctx, payID, user); err != nil {
		t.Fatalf("确认冲抵收款失败: %v", err)
	}
	after, err := st.GetAccount(ctx, acc.ID)
	if err != nil {
		t.Fatalf("查询账户失败: %v", err)
	}
	if after.Balance != acc.Balance {
		t.Fatalf("冲抵不应改变账户余额：确认前 %s，确认后 %s", acc.Balance, after.Balance)
	}
	flows, err := st.CashFlow(ctx, "", "", "", 0, 100)
	if err != nil {
		t.Fatalf("查询资金流水失败: %v", err)
	}
	if len(flows) != 0 {
		t.Fatalf("冲抵不应进入资金流水，实际 %d 条", len(flows))
	}

	// 作废冲抵后账户仍不变；应收恢复。
	if err := st.VoidPayment(ctx, payID, user); err != nil {
		t.Fatalf("作废冲抵收款失败: %v", err)
	}
	afterVoid, _ := st.GetAccount(ctx, acc.ID)
	if afterVoid.Balance != acc.Balance {
		t.Fatalf("作废冲抵不应改变账户余额：期望 %s，实际 %s", acc.Balance, afterVoid.Balance)
	}
	if ar, _, _ := st.PartnerBalance(ctx, customer); ar != 0 {
		t.Fatalf("作废冲抵后应收期望 0，实际 %s", ar)
	}

	// 普通收款仍然改变余额并进入资金流水。
	normalID, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: acc.ID,
		PayDate: model.Today(), Amount: model.YuanFrom(10), Method: "bank",
	}, user)
	if err != nil {
		t.Fatalf("创建普通收款失败: %v", err)
	}
	if err := st.ConfirmPayment(ctx, normalID, user); err != nil {
		t.Fatalf("确认普通收款失败: %v", err)
	}
	afterNormal, _ := st.GetAccount(ctx, acc.ID)
	if afterNormal.Balance != acc.Balance+model.YuanFrom(10) {
		t.Fatalf("普通收款后余额期望 %s，实际 %s", acc.Balance+model.YuanFrom(10), afterNormal.Balance)
	}
	flows, _ = st.CashFlow(ctx, "", "", "", 0, 100)
	if len(flows) != 1 {
		t.Fatalf("普通收款应进入资金流水，实际 %d 条", len(flows))
	}
}

// P0-3：作废已被后续业务消耗的采购单时，必须拒绝并完整回滚。
func TestP0VoidPurchaseBlockedWhenStockConsumed(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	so, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, so, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	if err := st.VoidPurchase(ctx, po, user); err == nil {
		t.Fatalf("库存已耗尽时不应允许作废采购单")
	} else if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("作废采购单应返回参数错误，实际 %v", err)
	}
	got, err := st.GetPurchase(ctx, po)
	if err != nil {
		t.Fatalf("查询采购单失败: %v", err)
	}
	if got.Status != model.DocConfirmed {
		t.Fatalf("作废失败后采购单应保持已确认，实际状态 %d", got.Status)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 0 {
		t.Fatalf("作废失败后库存应保持 0，实际 %s", q)
	}
}

// P0-3：作废调拨时，调入仓已被消耗也必须拒绝。
func TestP0VoidTransferBlockedWhenDestinationConsumed(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)
	w2, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "P0-WH2", Name: "P0 二仓", Status: 1})
	if err != nil {
		t.Fatalf("创建仓库失败: %v", err)
	}

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	tr, err := st.SaveTransfer(ctx, 0, TransferInput{
		FromWarehouseID: wid, ToWarehouseID: w2, TransferDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建调拨单失败: %v", err)
	}
	if err := st.ConfirmTransfer(ctx, tr, user); err != nil {
		t.Fatalf("确认调拨失败: %v", err)
	}
	so, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: w2, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, so, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	if err := st.VoidTransfer(ctx, tr, user); err == nil {
		t.Fatalf("调入仓库存已耗尽时不应允许作废调拨")
	} else if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("作废调拨应返回参数错误，实际 %v", err)
	}
	got, err := st.GetTransfer(ctx, tr)
	if err != nil {
		t.Fatalf("查询调拨单失败: %v", err)
	}
	if got.Status != model.DocConfirmed {
		t.Fatalf("作废失败后调拨单应保持已确认，实际状态 %d", got.Status)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 0 {
		t.Fatalf("失败回滚后调出仓库存应保持 0，实际 %s", q)
	}
	if q, _ := stockQty(t, st, w2, pid); q != 0 {
		t.Fatalf("失败回滚后调入仓库存应保持 0，实际 %s", q)
	}
}

// P0-3：作废盘点时，若之后的出库已经消耗了盘盈数量，也必须拒绝。
func TestP0VoidStockTakeBlockedWhenStockConsumed(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	takeID, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建盘点单失败: %v", err)
	}
	if err := st.FinishStockTake(ctx, takeID, user); err != nil {
		t.Fatalf("完成盘点失败: %v", err)
	}
	so, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(15), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, so, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	if err := st.VoidStockTake(ctx, takeID, user); err == nil {
		t.Fatalf("盘盈已被消耗时不应允许作废盘点")
	} else if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("作废盘点应返回参数错误，实际 %v", err)
	}
	got, err := st.GetStockTake(ctx, takeID)
	if err != nil {
		t.Fatalf("查询盘点单失败: %v", err)
	}
	if got.Status != model.DocConfirmed {
		t.Fatalf("作废失败后盘点单应保持已完成，实际状态 %d", got.Status)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 5 {
		t.Fatalf("作废失败后库存应保持 5，实际 %s", q)
	}
}

// P0-4：内置角色的权限修改必须跨重启保持。
func TestP0RolePermissionsSurviveRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	roles, err := st.ListRoles(ctx)
	if err != nil {
		t.Fatalf("查询角色失败: %v", err)
	}
	var manager model.Role
	for _, r := range roles {
		if r.Code == "manager" {
			manager = r
		}
	}
	if manager.ID == 0 {
		t.Fatal("未找到内置 manager 角色")
	}
	hasExport := false
	var perms []string
	for _, p := range manager.Perms {
		if p == "report:export" {
			hasExport = true
			continue
		}
		perms = append(perms, p)
	}
	if !hasExport {
		t.Fatal("manager 初始应拥有 report:export")
	}
	if err := st.UpdateRole(ctx, manager.ID, RoleInput{
		Name: manager.Name, Remark: manager.Remark, Perms: perms,
	}); err != nil {
		t.Fatalf("移除权限失败: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("关闭数据库失败: %v", err)
	}

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("重新打开数据库失败: %v", err)
	}
	defer st2.Close()
	got, err := st2.GetRole(ctx, manager.ID)
	if err != nil {
		t.Fatalf("重新读取角色失败: %v", err)
	}
	for _, p := range got.Perms {
		if p == "report:export" {
			t.Fatal("重启后被移除的 report:export 不应被 seed 重新加回")
		}
	}
}

// P0-5：负单价、负优惠、负费用和负参考价必须被拒绝，且不产生数据。
func TestP0NegativeAmountsRejected(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	purchase := PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}
	badPrice := purchase
	badPrice.Items = []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(-1)}}
	if _, err := st.SavePurchase(ctx, 0, badPrice, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("负单价采购应被拒绝，实际 %v", err)
	}
	badDiscount := purchase
	badDiscount.DiscountAmount = model.YuanFrom(-1)
	if _, err := st.SavePurchase(ctx, 0, badDiscount, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("负优惠采购应被拒绝，实际 %v", err)
	}
	badFee := purchase
	badFee.OtherFee = model.YuanFrom(-1)
	if _, err := st.SavePurchase(ctx, 0, badFee, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("负其他费用采购应被拒绝，实际 %v", err)
	}
	if list, err := st.ListPurchases(ctx, PurchaseQuery{Status: -1, Page: 1, Size: 10}); err != nil || list.Total != 0 {
		t.Fatalf("失败的采购不应写入数据，实际 %d 条 / %v", list.Total, err)
	}

	sale := SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}
	badSalePrice := sale
	badSalePrice.Items = []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(-1)}}
	if _, err := st.SaveSales(ctx, 0, badSalePrice, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("负单价销售应被拒绝，实际 %v", err)
	}
	badSaleFee := sale
	badSaleFee.OtherFee = model.YuanFrom(-1)
	if _, err := st.SaveSales(ctx, 0, badSaleFee, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("负其他费用销售应被拒绝，实际 %v", err)
	}
	if list, err := st.ListSales(ctx, SalesQuery{Status: -1, Page: 1, Size: 10}); err != nil || list.Total != 0 {
		t.Fatalf("失败的销售不应写入数据，实际 %d 条 / %v", list.Total, err)
	}

	if _, err := st.SaveProduct(ctx, 0, ProductInput{Name: "负价商品", PurchasePrice: model.PriceFrom(-1)}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("负参考进价应被拒绝，实际 %v", err)
	}
	if _, err := st.SaveProduct(ctx, 0, ProductInput{Name: "负库存商品", SafetyStock: model.QtyFrom(-1)}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("负安全库存应被拒绝，实际 %v", err)
	}
}

// P0-6：本月统计不能包含上/下个月的单据。
func TestP0DashboardMonthExcludesAdjacentMonths(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}

	prev := time.Now().AddDate(0, -1, 0).Format(model.DateLayout)
	next := time.Now().AddDate(0, 1, 0).Format(model.DateLayout)
	for _, tc := range []struct {
		date  string
		price float64
	}{
		{prev, 100},
		{model.Today(), 200},
		{next, 400},
	} {
		so, err := st.SaveSales(ctx, 0, SalesInput{
			CustomerID: customer, WarehouseID: wid, OrderDate: tc.date,
			Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(tc.price)}},
		}, user)
		if err != nil {
			t.Fatalf("创建 %s 销售单失败: %v", tc.date, err)
		}
		if err := st.ConfirmSales(ctx, so, user); err != nil {
			t.Fatalf("确认 %s 销售单失败: %v", tc.date, err)
		}
	}

	d, err := st.Dashboard(ctx)
	if err != nil {
		t.Fatalf("查询看板失败: %v", err)
	}
	if d.MonthSales != model.YuanFrom(200) {
		t.Fatalf("本月销售期望 200 元，实际 %s", d.MonthSales)
	}
	if d.MonthPurchase != model.YuanFrom(100) {
		t.Fatalf("本月采购期望 100 元，实际 %s", d.MonthPurchase)
	}
	if d.MonthProfit != model.YuanFrom(190) {
		t.Fatalf("本月毛利期望 190 元，实际 %s", d.MonthProfit)
	}
}

// P0-7：未指定仓库时必须使用默认仓库 ID，不能写入 warehouse_id=0。
func TestP0DefaultWarehouseIDFilled(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: 0, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(5), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建默认仓库采购单失败: %v", err)
	}
	gotPO, err := st.GetPurchase(ctx, po)
	if err != nil {
		t.Fatalf("查询采购单失败: %v", err)
	}
	if gotPO.WarehouseID != wid {
		t.Fatalf("采购单仓库 ID 期望 %d，实际 %d", wid, gotPO.WarehouseID)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}

	so, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: 0, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建默认仓库销售单失败: %v", err)
	}
	gotSO, err := st.GetSales(ctx, so)
	if err != nil {
		t.Fatalf("查询销售单失败: %v", err)
	}
	if gotSO.WarehouseID != wid {
		t.Fatalf("销售单仓库 ID 期望 %d，实际 %d", wid, gotSO.WarehouseID)
	}
	if err := st.ConfirmSales(ctx, so, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	takeID, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: 0, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(4)}},
	}, user)
	if err != nil {
		t.Fatalf("创建默认仓库盘点单失败: %v", err)
	}
	gotTake, err := st.GetStockTake(ctx, takeID)
	if err != nil {
		t.Fatalf("查询盘点单失败: %v", err)
	}
	if gotTake.WarehouseID != wid {
		t.Fatalf("盘点单仓库 ID 期望 %d，实际 %d", wid, gotTake.WarehouseID)
	}
	if err := st.FinishStockTake(ctx, takeID, user); err != nil {
		t.Fatalf("完成盘点失败: %v", err)
	}

	var zeroStocks int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stocks WHERE warehouse_id=0`).Scan(&zeroStocks); err != nil {
		t.Fatalf("统计仓库 0 库存失败: %v", err)
	}
	if zeroStocks != 0 {
		t.Fatalf("不应存在 warehouse_id=0 的库存行，实际 %d 条", zeroStocks)
	}
	var zeroMoves int
	if err := st.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM stock_moves WHERE warehouse_id=0`).Scan(&zeroMoves); err != nil {
		t.Fatalf("统计仓库 0 流水失败: %v", err)
	}
	if zeroMoves != 0 {
		t.Fatalf("不应存在 warehouse_id=0 的库存流水，实际 %d 条", zeroMoves)
	}
}

// P0-8：有余额的账户/往来单位不能被删除。
func TestP0DeleteWithBalanceBlocked(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	accID, err := st.SaveAccount(ctx, 0, AccountInput{
		Code: "P0-ACC", Name: "有余额账户", Type: "cash", Balance: model.YuanFrom(100), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建有余额账户失败: %v", err)
	}
	if err := st.DeleteAccount(ctx, accID); !errors.Is(err, ErrInUse) {
		t.Fatalf("删除有余额账户应返回 ErrInUse，实际 %v", err)
	}
	if _, err := st.GetAccount(ctx, accID); err != nil {
		t.Fatalf("删除失败后账户应仍存在: %v", err)
	}

	emptyAcc, err := st.SaveAccount(ctx, 0, AccountInput{Code: "P0-ACC0", Name: "空账户", Type: "cash", Status: 1})
	if err != nil {
		t.Fatalf("创建空账户失败: %v", err)
	}
	if err := st.DeleteAccount(ctx, emptyAcc); err != nil {
		t.Fatalf("删除空账户失败: %v", err)
	}

	partnerID, err := st.SavePartner(ctx, 0, PartnerInput{
		Code: "P0-PAR", Name: "有期初余额客户", Type: "customer",
		InitReceivable: model.YuanFrom(100), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建有余额往来单位失败: %v", err)
	}
	if err := st.DeletePartner(ctx, partnerID); !errors.Is(err, ErrInUse) {
		t.Fatalf("删除有余额往来单位应返回 ErrInUse，实际 %v", err)
	}
	if _, err := st.GetPartner(ctx, partnerID); err != nil {
		t.Fatalf("删除失败后往来单位应仍存在: %v", err)
	}

	emptyPartner, err := st.SavePartner(ctx, 0, PartnerInput{
		Code: "P0-PAR0", Name: "无余额客户", Type: "customer", Status: 1,
	})
	if err != nil {
		t.Fatalf("创建无余额往来单位失败: %v", err)
	}
	if err := st.DeletePartner(ctx, emptyPartner); err != nil {
		t.Fatalf("删除无余额往来单位失败: %v", err)
	}
}
