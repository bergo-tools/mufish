package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mufish/internal/model"
)

func TestCatalogOptionQueriesAndBackup(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if st.Path() == "" {
		t.Fatalf("Store.Path 不应为空")
	}
	cats, err := st.AllCategories(ctx)
	if err != nil || len(cats) == 0 {
		t.Fatalf("查询全部分类失败: %d / %v", len(cats), err)
	}
	products, err := st.AllProducts(ctx, "SP001")
	if err != nil || len(products) != 1 {
		t.Fatalf("按关键字查询商品选项失败: %d / %v", len(products), err)
	}
	partners, err := st.AllPartners(ctx, "customer", "华南")
	if err != nil || len(partners) != 1 {
		t.Fatalf("按类型和关键字查询往来单位失败: %d / %v", len(partners), err)
	}
	settings, err := st.Settings(ctx)
	if err != nil || settings["company_name"] == "" {
		t.Fatalf("读取全部参数失败: %+v / %v", settings, err)
	}

	dest := filepath.Join(t.TempDir(), "backup.db")
	if err := st.BackupTo(ctx, dest); err != nil {
		t.Fatalf("备份数据库失败: %v", err)
	}
	if fi, err := os.Stat(dest); err != nil || fi.Size() == 0 {
		t.Fatalf("备份文件应存在且非空: %v / %v", fi, err)
	}
}

func TestDocListsAndFilters(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	p1, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if _, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: "2000-01-01",
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user); err != nil {
		t.Fatalf("创建历史采购单失败: %v", err)
	}
	s1, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}

	purchases, err := st.ListPurchases(ctx, PurchaseQuery{
		Keyword: "测试供应商", SupplierID: supplier, Status: 0,
		From: model.Today(), To: model.Today(), Page: 1, Size: 10,
	})
	if err != nil || purchases.Total != 1 || purchases.List[0].ID != p1 {
		t.Fatalf("采购单过滤结果不正确: %+v / %v", purchases, err)
	}
	sales, err := st.ListSales(ctx, SalesQuery{
		Keyword: "测试客户", CustomerID: customer, Status: 0,
		From: model.Today(), To: model.Today(), Page: 1, Size: 10,
	})
	if err != nil || sales.Total != 1 || sales.List[0].ID != s1 {
		t.Fatalf("销售单过滤结果不正确: %+v / %v", sales, err)
	}
	if err := st.ConfirmPurchase(ctx, p1, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	purchases, err = st.ListPurchases(ctx, PurchaseQuery{Status: 1, Page: 1, Size: 10})
	if err != nil || purchases.Total != 1 {
		t.Fatalf("已确认采购单过滤期望 1，实际 %d / %v", purchases.Total, err)
	}
	if err := st.VoidPurchase(ctx, p1, user); err != nil {
		t.Fatalf("作废采购单失败: %v", err)
	}
	purchases, err = st.ListPurchases(ctx, PurchaseQuery{Status: 2, Page: 1, Size: 10})
	if err != nil || purchases.Total != 1 {
		t.Fatalf("已作废采购单过滤期望 1，实际 %d / %v", purchases.Total, err)
	}
}

func TestPaymentQueriesUpdateAndDelete(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	purchase, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, purchase, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	accounts, err := st.ListAccounts(ctx, true)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("应存在资金账户: %v", err)
	}
	acc := accounts[0]

	if _, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: "bad", PartnerID: supplier, AccountID: acc.ID, PayDate: model.Today(), Amount: 1,
	}, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("非法收付款类型应返回 ErrBadRequest，实际 %v", err)
	}
	if _, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypePayment, PartnerID: supplier, AccountID: acc.ID, PayDate: model.Today(), Amount: 0,
	}, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("金额为 0 应返回 ErrBadRequest，实际 %v", err)
	}
	if _, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypePayment, PartnerID: supplier, AccountID: 999999, PayDate: model.Today(), Amount: 1,
	}, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("不存在的账户应返回 ErrBadRequest，实际 %v", err)
	}

	paymentID, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypePayment, PartnerID: supplier, AccountID: acc.ID,
		PayDate: model.Today(), Amount: model.YuanFrom(30), Method: "bank",
		OrderType: "purchase", OrderID: purchase,
	}, user)
	if err != nil {
		t.Fatalf("创建付款单失败: %v", err)
	}
	p, err := st.GetPayment(ctx, paymentID)
	if err != nil || p.PartnerName != "测试供应商" || p.OrderNo == "" {
		t.Fatalf("读取付款单不正确: %+v / %v", p, err)
	}
	list, err := st.ListPayments(ctx, PaymentQuery{
		Keyword: "测试供应商", Type: model.PayTypePayment, PartnerID: supplier,
		AccountID: acc.ID, Status: 0, From: model.Today(), To: model.Today(), Page: 1, Size: 10,
	})
	if err != nil || list.Total != 1 {
		t.Fatalf("付款单过滤期望 1，实际 %d / %v", list.Total, err)
	}

	// 修改草稿。
	if _, err := st.SavePayment(ctx, paymentID, PaymentInput{
		Type: model.PayTypePayment, PartnerID: supplier, AccountID: acc.ID,
		PayDate: model.Today(), Amount: model.YuanFrom(20), Method: "cash",
		OrderType: "purchase", OrderID: purchase,
	}, user); err != nil {
		t.Fatalf("修改草稿付款单失败: %v", err)
	}
	p, _ = st.GetPayment(ctx, paymentID)
	if p.Amount != model.YuanFrom(20) || p.Method != "cash" {
		t.Fatalf("修改付款单未生效: %+v", p)
	}

	if err := st.ConfirmPayment(ctx, paymentID, user); err != nil {
		t.Fatalf("确认付款单失败: %v", err)
	}
	if err := st.ConfirmPayment(ctx, paymentID, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("重复确认应返回 ErrBadRequest，实际 %v", err)
	}
	if err := st.DeletePayment(ctx, paymentID); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("删除已确认付款单应返回 ErrBadRequest，实际 %v", err)
	}
	if err := st.VoidPayment(ctx, paymentID, user); err != nil {
		t.Fatalf("作废付款单失败: %v", err)
	}
	if err := st.VoidPayment(ctx, paymentID, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("重复作废应返回 ErrBadRequest，实际 %v", err)
	}

	draftID, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: acc.ID,
		PayDate: model.Today(), Amount: model.YuanFrom(5),
	}, user)
	if err != nil {
		t.Fatalf("创建草稿收款单失败: %v", err)
	}
	if err := st.DeletePayment(ctx, draftID); err != nil {
		t.Fatalf("删除草稿收款单失败: %v", err)
	}
	if err := st.DeletePayment(ctx, draftID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除收款单应返回 ErrNotFound，实际 %v", err)
	}
	if _, err := st.GetPayment(ctx, draftID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("读取已删除收款单应返回 ErrNotFound，实际 %v", err)
	}
}

func TestStockTakeAndTransferListsAndDeletes(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)

	purchase, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(100), Price: model.PriceFrom(8)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, purchase, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}

	takeID, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(100)}},
	}, user)
	if err != nil {
		t.Fatalf("创建盘点单失败: %v", err)
	}
	takes, err := st.ListStockTakes(ctx, StockTakeQuery{
		Keyword: "PD", WarehouseID: wid, Status: 0,
		From: model.Today(), To: model.Today(), Page: 1, Size: 10,
	})
	if err != nil || takes.Total != 1 || takes.List[0].ID != takeID {
		t.Fatalf("盘点单过滤不正确: %+v / %v", takes, err)
	}
	if _, err := st.GetStockTake(ctx, takeID); err != nil {
		t.Fatalf("读取盘点单失败: %v", err)
	}
	if err := st.DeleteStockTake(ctx, takeID); err != nil {
		t.Fatalf("删除草稿盘点单失败: %v", err)
	}
	if err := st.DeleteStockTake(ctx, takeID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除盘点单应返回 ErrNotFound，实际 %v", err)
	}

	takeID2, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(98)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第二张盘点单失败: %v", err)
	}
	if err := st.FinishStockTake(ctx, takeID2, user); err != nil {
		t.Fatalf("完成盘点失败: %v", err)
	}
	if err := st.DeleteStockTake(ctx, takeID2); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("删除已确认盘点单应返回 ErrBadRequest，实际 %v", err)
	}
	if err := st.VoidStockTake(ctx, takeID2, user); err != nil {
		t.Fatalf("作废盘点单失败: %v", err)
	}
	if err := st.VoidStockTake(ctx, takeID2, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("重复作废盘点单应返回 ErrBadRequest，实际 %v", err)
	}
	takes, err = st.ListStockTakes(ctx, StockTakeQuery{Status: 2, Page: 1, Size: 10})
	if err != nil || takes.Total != 1 {
		t.Fatalf("已作废盘点单期望 1，实际 %d / %v", takes.Total, err)
	}

	w2, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "LIST-W2", Name: "列表测试二仓", Status: 1})
	if err != nil {
		t.Fatalf("创建仓库失败: %v", err)
	}
	trID, err := st.SaveTransfer(ctx, 0, TransferInput{
		FromWarehouseID: wid, ToWarehouseID: w2, TransferDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(8)}},
	}, user)
	if err != nil {
		t.Fatalf("创建调拨单失败: %v", err)
	}
	transfers, err := st.ListTransfers(ctx, TransferQuery{
		Keyword: "DB", Status: 0, From: model.Today(), To: model.Today(), Page: 1, Size: 10,
	})
	if err != nil || transfers.Total != 1 || transfers.List[0].ID != trID {
		t.Fatalf("调拨单过滤不正确: %+v / %v", transfers, err)
	}
	if _, err := st.GetTransfer(ctx, trID); err != nil {
		t.Fatalf("读取调拨单失败: %v", err)
	}
	if err := st.DeleteTransfer(ctx, trID); err != nil {
		t.Fatalf("删除草稿调拨单失败: %v", err)
	}
	if err := st.DeleteTransfer(ctx, trID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("重复删除调拨单应返回 ErrNotFound，实际 %v", err)
	}

	trID2, err := st.SaveTransfer(ctx, 0, TransferInput{
		FromWarehouseID: wid, ToWarehouseID: w2, TransferDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(8)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第二张调拨单失败: %v", err)
	}
	if err := st.ConfirmTransfer(ctx, trID2, user); err != nil {
		t.Fatalf("确认调拨单失败: %v", err)
	}
	if err := st.DeleteTransfer(ctx, trID2); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("删除已确认调拨单应返回 ErrBadRequest，实际 %v", err)
	}
	if err := st.VoidTransfer(ctx, trID2, user); err != nil {
		t.Fatalf("作废调拨单失败: %v", err)
	}
	if err := st.VoidTransfer(ctx, trID2, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("重复作废调拨单应返回 ErrBadRequest，实际 %v", err)
	}
	transfers, err = st.ListTransfers(ctx, TransferQuery{Status: 2, Page: 1, Size: 10})
	if err != nil || transfers.Total != 1 {
		t.Fatalf("已作废调拨单期望 1，实际 %d / %v", transfers.Total, err)
	}
}

func TestUserListUpdateAndCleanSessions(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	users, err := st.ListUsers(ctx, "admin", 1, 10)
	if err != nil || users.Total != 1 {
		t.Fatalf("用户关键字查询期望 1，实际 %d / %v", users.Total, err)
	}
	users, err = st.ListUsers(ctx, "", 1, 2)
	if err != nil || len(users.List) != 2 {
		t.Fatalf("用户分页期望 2，实际 %d / %v", len(users.List), err)
	}

	uid, err := st.CreateUser(ctx, UserInput{Username: "list_user", Password: "abc123456", Name: "列表用户", Status: 1})
	if err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	// 带口令更新走独立分支。
	if err := st.UpdateUser(ctx, uid, UserInput{
		Username: "list_user", Password: "newpass123", Name: "列表用户（改）", Status: 1,
	}); err != nil {
		t.Fatalf("更新用户口令失败: %v", err)
	}
	if _, _, err := st.Authenticate(ctx, "list_user", "newpass123"); err != nil {
		t.Fatalf("更新后的口令应可登录: %v", err)
	}
	if err := st.UpdateUser(ctx, 999999, UserInput{Name: "不存在"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("更新不存在用户应返回 ErrNotFound，实际 %v", err)
	}

	token, _, err := st.CreateSession(ctx, uid, "127.0.0.1", "test", -time.Hour)
	if err != nil {
		t.Fatalf("创建过期会话失败: %v", err)
	}
	st.CleanSessions(ctx)
	var count int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE token=?`, token).Scan(&count); err != nil {
		t.Fatalf("查询会话数量失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("CleanSessions 应删除过期会话，实际剩余 %d", count)
	}
	if err := st.DeleteUser(ctx, uid); err != nil {
		t.Fatalf("删除用户失败: %v", err)
	}
}
