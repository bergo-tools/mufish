package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mufish/internal/model"
)

// P1-6：修改口令后所有旧会话必须失效。
func TestP1PasswordChangeRevokesSessions(t *testing.T) {
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
		t.Fatal("新会话应有效")
	}
	if err := st.UpdatePassword(ctx, u.ID, DefaultAdminPassword, "newpass123"); err != nil {
		t.Fatalf("修改口令失败: %v", err)
	}
	if _, ok := st.SessionUser(ctx, token); ok {
		t.Fatal("修改口令后旧会话应失效")
	}
	if _, _, err := st.Authenticate(ctx, "admin", "newpass123"); err != nil {
		t.Fatalf("新口令应可登录: %v", err)
	}
}

// P1-8：看板资金余额与账户页一致，应统计停用账户里的余额。
func TestP1DashboardAccountTotalIncludesDisabled(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	if _, err := st.SaveAccount(ctx, 0, AccountInput{
		Code: "P1-DISABLED", Name: "停用账户", Type: "cash",
		Balance: model.YuanFrom(100), Status: 0,
	}); err != nil {
		t.Fatalf("创建停用账户失败: %v", err)
	}
	d, err := st.Dashboard(ctx)
	if err != nil {
		t.Fatalf("查询看板失败: %v", err)
	}
	if d.AccountTotal != model.YuanFrom(100) {
		t.Fatalf("资金余额应包含停用账户余额 100 元，实际 %s", d.AccountTotal)
	}
}

// P1-11：购销明细报表必须带单据编号与日期。
func TestP1ReportDetailsIncludeDocumentInfo(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	gotPO, _ := st.GetPurchase(ctx, po)
	purchaseRows, err := st.PurchaseDetail(ctx, DocDetailQuery{From: model.Today(), To: model.Today()})
	if err != nil || len(purchaseRows) != 1 {
		t.Fatalf("查询采购明细失败: %d 条 / %v", len(purchaseRows), err)
	}
	if purchaseRows[0].DocNo != gotPO.OrderNo || purchaseRows[0].DocDate != gotPO.OrderDate {
		t.Fatalf("采购明细单据信息不正确：%q %q，期望 %q %q",
			purchaseRows[0].DocNo, purchaseRows[0].DocDate, gotPO.OrderNo, gotPO.OrderDate)
	}

	so, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, so, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}
	gotSO, _ := st.GetSales(ctx, so)
	salesRows, err := st.SalesDetail(ctx, DocDetailQuery{From: model.Today(), To: model.Today()})
	if err != nil || len(salesRows) != 1 {
		t.Fatalf("查询销售明细失败: %d 条 / %v", len(salesRows), err)
	}
	if salesRows[0].DocNo != gotSO.OrderNo || salesRows[0].DocDate != gotSO.OrderDate {
		t.Fatalf("销售明细单据信息不正确：%q %q，期望 %q %q",
			salesRows[0].DocNo, salesRows[0].DocDate, gotSO.OrderNo, gotSO.OrderDate)
	}
}

// P1-23：演示数据只初始化一次。
func TestP1DemoSeedOnlyOnce(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	if got := st.Setting(ctx, "demo_seeded"); got != "1" {
		t.Fatalf("demo_seeded 应为 1，实际 %q", got)
	}
	page, err := st.ListProducts(ctx, ProductQuery{Status: -1, Page: 1, Size: 100})
	if err != nil {
		t.Fatalf("查询商品失败: %v", err)
	}
	firstCount := page.Total
	if firstCount == 0 {
		t.Fatal("首次初始化应写入演示商品")
	}
	if err := st.Close(); err != nil {
		t.Fatalf("关闭数据库失败: %v", err)
	}
	st2, err := Open(path)
	if err != nil {
		t.Fatalf("重新打开数据库失败: %v", err)
	}
	defer st2.Close()
	page2, err := st2.ListProducts(ctx, ProductQuery{Status: -1, Page: 1, Size: 100})
	if err != nil {
		t.Fatalf("重新打开后查询商品失败: %v", err)
	}
	if page2.Total != firstCount {
		t.Fatalf("重启后演示商品数量不应变化：首次 %d，重启后 %d", firstCount, page2.Total)
	}
}
