package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"mufish/internal/model"
	"mufish/internal/store"
)

// P1-6：普通 user:manage 用户不能重置/修改超级管理员。
func TestP1NonSuperCannotResetSuperAdmin(t *testing.T) {
	c, st := newTestServer(t)
	c.login("admin", "admin123")
	admin, err := st.GetUserByName(context.Background(), "admin")
	if err != nil {
		t.Fatalf("查询管理员失败: %v", err)
	}

	role := decode[map[string]any](t, c.do("POST", "/roles", map[string]any{
		"code": "p1_user_manager", "name": "P1 用户管理员",
		"perms": []string{"user:manage"},
	}, 0))
	roleID := role["id"].(float64)
	user := decode[map[string]any](t, c.do("POST", "/users", map[string]any{
		"username": "p1_user_admin", "password": "abc123456", "name": "P1 用户管理员",
		"role_id": roleID, "status": 1,
	}, 0))
	if user["id"] == nil {
		t.Fatal("创建测试用户失败")
	}

	c.login("p1_user_admin", "abc123456")
	c.do("PUT", "/users/"+strconv.FormatInt(admin.ID, 10), map[string]any{
		"username": "admin", "name": "被篡改", "role_id": admin.RoleID,
		"status": 1, "password": "hacked123",
	}, http.StatusForbidden)
	c.do("POST", "/users/"+strconv.FormatInt(admin.ID, 10)+"/reset-password", map[string]any{
		"password": "hacked123",
	}, http.StatusForbidden)

	// 原口令必须仍然有效。
	if _, _, err := st.Authenticate(context.Background(), "admin", "admin123"); err != nil {
		t.Fatalf("越权请求后管理员口令不应被修改: %v", err)
	}
}

// P1-4：共享下拉接口不能脱离模块权限自由读取。
func TestP1OptionEndpointsRequirePermissions(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")
	role := decode[map[string]any](t, c.do("POST", "/roles", map[string]any{
		"code": "p1_report_only", "name": "P1 报表用户",
		"perms": []string{"report:view"},
	}, 0))
	roleID := role["id"].(float64)
	c.do("POST", "/users", map[string]any{
		"username": "p1_report_user", "password": "abc123456", "name": "P1 报表用户",
		"role_id": roleID, "status": 1,
	}, 0)

	c.login("p1_report_user", "abc123456")
	// 报表页需要仓库/客户/供应商/账户选项。
	c.do("GET", "/warehouses/options", nil, 0)
	c.do("GET", "/partners/options", nil, 0)
	c.do("GET", "/accounts/options", nil, 0)
	// 商品选项、未结单据和系统信息不属于报表页所需数据。
	c.do("GET", "/products/options", nil, http.StatusForbidden)
	c.do("GET", "/payments/unpaid-orders?type=sales&partner_id=1", nil, http.StatusForbidden)
	c.do("GET", "/system/info", nil, http.StatusForbidden)
}

// P1-10：看板「最近单据」必须全局按日期排序，而不是先销售后采购。
func TestP1DashboardRecentSortedByDate(t *testing.T) {
	c, st := newTestServer(t)
	c.login("admin", "admin123")
	ctx := context.Background()
	products, err := st.AllProducts(ctx, "")
	if err != nil || len(products) == 0 {
		t.Fatalf("查询商品失败: %v", err)
	}
	suppliers, _ := st.AllPartners(ctx, "supplier", "")
	customers, _ := st.AllPartners(ctx, "customer", "")
	if len(suppliers) == 0 || len(customers) == 0 {
		t.Fatal("缺少测试往来单位")
	}
	wh, _ := st.DefaultWarehouse(ctx)
	productID := products[0].ID
	user := store.OpUser{ID: 1, Name: "测试员"}
	if _, err := st.SavePurchase(ctx, 0, store.PurchaseInput{
		SupplierID: suppliers[0].ID, WarehouseID: wh.ID, OrderDate: "2000-01-01",
		Items: []store.OrderItemInput{{ProductID: productID, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user); err != nil {
		t.Fatalf("创建旧采购单失败: %v", err)
	}
	if _, err := st.SaveSales(ctx, 0, store.SalesInput{
		CustomerID: customers[0].ID, WarehouseID: wh.ID, OrderDate: "2000-02-01",
		Items: []store.OrderItemInput{{ProductID: productID, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user); err != nil {
		t.Fatalf("创建中间销售单失败: %v", err)
	}
	if _, err := st.SavePurchase(ctx, 0, store.PurchaseInput{
		SupplierID: suppliers[0].ID, WarehouseID: wh.ID, OrderDate: "2000-03-01",
		Items: []store.OrderItemInput{{ProductID: productID, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user); err != nil {
		t.Fatalf("创建新采购单失败: %v", err)
	}

	data := decode[map[string]any](t, c.do("GET", "/dashboard", nil, 0))
	recent, _ := data["recent"].([]any)
	if len(recent) < 3 {
		t.Fatalf("最近单据期望至少 3 条，实际 %d", len(recent))
	}
	for i := 0; i < len(recent)-1; i++ {
		cur := recent[i].(map[string]any)["date"].(string)
		next := recent[i+1].(map[string]any)["date"].(string)
		if cur < next {
			t.Fatalf("最近单据未按日期倒序：%q 在 %q 之前", cur, next)
		}
	}
}

// P1-5：只有 dashboard:view 的用户不能拿到销售/采购/库存/资金数据。
func TestP1DashboardRespectsModulePermissions(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")
	role := decode[map[string]any](t, c.do("POST", "/roles", map[string]any{
		"code": "p1_dashboard_only", "name": "P1 仅看板",
		"perms": []string{"dashboard:view"},
	}, 0))
	roleID := role["id"].(float64)
	c.do("POST", "/users", map[string]any{
		"username": "p1_dashboard_user", "password": "abc123456", "name": "P1 仅看板",
		"role_id": roleID, "status": 1,
	}, 0)

	c.login("p1_dashboard_user", "abc123456")
	data := decode[map[string]any](t, c.do("GET", "/dashboard", nil, 0))
	visibility := data["visibility"].(map[string]any)
	for _, key := range []string{"sales", "purchase", "stock", "finance", "account", "product", "partner"} {
		if visibility[key] != false {
			t.Fatalf("看板可见性 %s 期望 false，实际 %v", key, visibility[key])
		}
	}
	summary := data["summary"].(map[string]any)
	for _, key := range []string{"month_sales", "month_purchase", "month_profit", "stock_amount", "receivable", "payable", "account_total"} {
		if summary[key].(float64) != 0 {
			t.Fatalf("看板汇总 %s 期望 0，实际 %v", key, summary[key])
		}
	}
	if len(data["recent"].([]any)) != 0 {
		t.Fatalf("无销售/采购权限时最近单据应为空，实际 %v", data["recent"])
	}
	if len(data["alerts"].([]any)) != 0 {
		t.Fatalf("无库存权限时预警应为空，实际 %v", data["alerts"])
	}
	if len(data["trend"].([]any)) != 0 {
		t.Fatalf("无销售/采购权限时走势应为空，实际 %v", data["trend"])
	}
}

// P1-20：账户余额和往来单位期初金额的调整必须记录旧值/新值。
func TestP1BalanceChangesAreAudited(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	acc := decode[map[string]any](t, c.do("POST", "/accounts", map[string]any{
		"code": "P1-AUDIT-ACC", "name": "审计账户", "type": "cash",
		"balance": 100, "status": 1,
	}, 0))
	accID := strconv.FormatInt(int64(acc["id"].(float64)), 10)
	c.do("PUT", "/accounts/"+accID, map[string]any{
		"code": "P1-AUDIT-ACC", "name": "审计账户", "type": "cash",
		"balance": 200, "status": 1,
	}, 0)
	logs := decode[map[string]any](t, c.do("GET", "/logs?module=资金账户&keyword=余额&size=10", nil, 0))
	var accountDetail string
	for _, item := range logs["list"].([]any) {
		accountDetail += item.(map[string]any)["detail"].(string)
	}
	if !strings.Contains(accountDetail, "余额 100.00 → 200.00") {
		t.Fatalf("账户余额调整审计缺少旧值/新值: %s", accountDetail)
	}

	partner := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "P1-AUDIT-PAR", "name": "审计客户", "type": "customer",
		"init_receivable": 100, "status": 1,
	}, 0))
	partnerID := strconv.FormatInt(int64(partner["id"].(float64)), 10)
	c.do("PUT", "/partners/"+partnerID, map[string]any{
		"code": "P1-AUDIT-PAR", "name": "审计客户", "type": "customer",
		"init_receivable": 150, "status": 1,
	}, 0)
	logs = decode[map[string]any](t, c.do("GET", "/logs?module=往来单位&keyword=期初应收&size=10", nil, 0))
	var partnerDetail string
	for _, item := range logs["list"].([]any) {
		partnerDetail += item.(map[string]any)["detail"].(string)
	}
	if !strings.Contains(partnerDetail, "期初应收 100.00 → 150.00") {
		t.Fatalf("期初应收调整审计缺少旧值/新值: %s", partnerDetail)
	}
}

// P1-21：登录保持时长必须是整数。

// P1-18/19：CSV 导入错误行必须明确失败，导出单元格做公式注入转义。
func TestP1CSVImportValidationAndSafeExport(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	text := "商品编码,商品名称,规格,单位,分类,条码,品牌,参考进价,参考售价,安全库存\n" +
		"P1-OK,正常商品,规格,个,,,,10,20,5\n" +
		"P1-BAD,价格错误,规格,个,,,,abc,20,5\n" +
		"P1-EMPTY,,规格,个,,,,10,20,5\n"
	res := decode[map[string]any](t, c.do("POST", "/products/import", map[string]any{"text": text}, 0))
	if res["success"].(float64) != 1 {
		t.Fatalf("导入成功数期望 1，实际 %v", res["success"])
	}
	if res["failed"].(float64) != 2 {
		t.Fatalf("导入失败数期望 2，实际 %v", res["failed"])
	}
	msgs, _ := res["messages"].([]any)
	var joined string
	for _, m := range msgs {
		joined += m.(string)
	}
	if !strings.Contains(joined, "参考进价") || !strings.Contains(joined, "商品名称不能为空") {
		t.Fatalf("导入失败提示不完整: %s", joined)
	}

	if got := csvSafe("=1+1"); got != "'=1+1" {
		t.Fatalf("公式单元格应转义，实际 %q", got)
	}
	if got := csvSafe("-12.50"); got != "-12.50" {
		t.Fatalf("负数金额不应被转义，实际 %q", got)
	}
	if got := csvSafe("-2+2"); got != "'-2+2" {
		t.Fatalf("负数开头的公式应转义，实际 %q", got)
	}
	if got := csvSafe("正常备注"); got != "正常备注" {
		t.Fatalf("普通文本不应被修改，实际 %q", got)
	}
}

func TestP1SessionHoursValidation(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")
	c.do("PUT", "/settings", map[string]any{"session_hours": "1.5"}, http.StatusBadRequest)
	c.do("PUT", "/settings", map[string]any{"session_hours": "12"}, 0)
}
