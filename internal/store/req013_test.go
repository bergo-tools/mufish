package store

import (
	"context"
	"errors"
	"testing"

	"mufish/internal/model"
)

func disablePartnerForTest(t *testing.T, st *Store, id int64) {
	t.Helper()
	p, err := st.GetPartner(context.Background(), id)
	if err != nil {
		t.Fatalf("读取往来单位失败: %v", err)
	}
	if _, err := st.SavePartner(context.Background(), id, PartnerInput{
		Code: p.Code, Name: p.Name, Type: p.Type, Contact: p.Contact, Phone: p.Phone,
		Address: p.Address, TaxNo: p.TaxNo, BankName: p.BankName, BankAccount: p.BankAccount,
		CreditLimit: p.CreditLimit, InitReceivable: p.InitReceivable, InitPayable: p.InitPayable,
		Status: 0, Remark: p.Remark,
	}); err != nil {
		t.Fatalf("停用往来单位失败: %v", err)
	}
}

func disableAccountForTest(t *testing.T, st *Store, id int64) {
	t.Helper()
	a, err := st.GetAccount(context.Background(), id)
	if err != nil {
		t.Fatalf("读取资金账户失败: %v", err)
	}
	if _, err := st.SaveAccount(context.Background(), id, AccountInput{
		Code: a.Code, Name: a.Name, Type: a.Type, IsDefault: a.IsDefault,
		Sort: a.Sort, Status: 0, Remark: a.Remark,
	}); err != nil {
		t.Fatalf("停用资金账户失败: %v", err)
	}
}

func tableCount(t *testing.T, st *Store, table string) int {
	t.Helper()
	var n int
	if err := st.db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("统计 %s 失败: %v", table, err)
	}
	return n
}

// 停用的供应商/客户不能用于新采购、销售或收付款单；失败时不能留下单据。
func TestDisabledPartnerRejectedForNewDocuments(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	disablePartnerForTest(t, st, supplier)
	if _, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(1)}},
	}, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("停用供应商不应能新建采购单，实际 %v", err)
	}
	if n := tableCount(t, st, "purchase_orders"); n != 0 {
		t.Fatalf("停用供应商的采购单应被拒绝，实际写入 %d 条", n)
	}

	disablePartnerForTest(t, st, customer)
	if _, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(1)}},
	}, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("停用客户不应能新建销售单，实际 %v", err)
	}
	if n := tableCount(t, st, "sales_orders"); n != 0 {
		t.Fatalf("停用客户的销售单应被拒绝，实际写入 %d 条", n)
	}

	accID, err := st.SaveAccount(ctx, 0, AccountInput{Code: "REQ013-ACC", Name: "测试账户", Status: 1})
	if err != nil {
		t.Fatalf("创建账户失败: %v", err)
	}
	if _, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: accID,
		PayDate: model.Today(), Amount: model.YuanFrom(10), Method: "bank",
	}, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("停用客户不应能新建收款单，实际 %v", err)
	}
	if n := tableCount(t, st, "payments"); n != 0 {
		t.Fatalf("停用客户的收款单应被拒绝，实际写入 %d 条", n)
	}
}

// 停用账户不能用于需要资金账户的新收付款单；往来冲抵不需要账户，仍应可用。
func TestDisabledAccountRejectedForNewPaymentButOffsetWorks(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	_, _, customer, _ := bootstrap(t, st)

	accID, err := st.SaveAccount(ctx, 0, AccountInput{Code: "REQ013-DIS", Name: "停用账户", Status: 0})
	if err != nil {
		t.Fatalf("创建停用账户失败: %v", err)
	}
	if _, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: accID,
		PayDate: model.Today(), Amount: model.YuanFrom(10), Method: "bank",
	}, user); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("停用账户不应能新建收款单，实际 %v", err)
	}
	if n := tableCount(t, st, "payments"); n != 0 {
		t.Fatalf("停用账户的收款单应被拒绝，实际写入 %d 条", n)
	}

	if _, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: 0,
		PayDate: model.Today(), Amount: model.YuanFrom(10), Method: "offset",
	}, user); err != nil {
		t.Fatalf("往来冲抵不需要资金账户，不应被账户停用影响: %v", err)
	}
}

// 停用前已保存的草稿仍应可以确认，不能因为主数据停用而锁死历史单据。
func TestDraftPaymentCanConfirmAfterMasterDataDisabled(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	_, _, customer, _ := bootstrap(t, st)

	accID, err := st.SaveAccount(ctx, 0, AccountInput{Code: "REQ013-DRAFT", Name: "草稿账户", Status: 1})
	if err != nil {
		t.Fatalf("创建账户失败: %v", err)
	}
	payID, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: accID,
		PayDate: model.Today(), Amount: model.YuanFrom(100), Method: "bank",
	}, user)
	if err != nil {
		t.Fatalf("创建草稿收款单失败: %v", err)
	}

	disablePartnerForTest(t, st, customer)
	disableAccountForTest(t, st, accID)

	if err := st.ConfirmPayment(ctx, payID, user); err != nil {
		t.Fatalf("主数据停用后历史草稿仍应能确认: %v", err)
	}
	acc, err := st.GetAccount(ctx, accID)
	if err != nil {
		t.Fatalf("读取账户失败: %v", err)
	}
	if acc.Balance != model.YuanFrom(100) {
		t.Fatalf("确认后账户余额期望 100 元，实际 %s", acc.Balance)
	}
}
