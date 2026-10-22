package api

import (
	"context"
	"testing"

	"mufish/internal/model"
	"mufish/internal/store"
)

// 看板走势接口必须与看板首页使用相同的模块权限口径，不能泄露销售/采购数据。
func TestP1DashboardTrendRespectsModulePermissions(t *testing.T) {
	c, st := newTestServer(t)
	ctx := context.Background()
	user := store.OpUser{ID: 1, Name: "测试员"}

	products, err := st.AllProducts(ctx, "")
	if err != nil || len(products) == 0 {
		t.Fatalf("查询商品失败: %v", err)
	}
	suppliers, _ := st.AllPartners(ctx, "supplier", "")
	customers, _ := st.AllPartners(ctx, "customer", "")
	if len(suppliers) == 0 || len(customers) == 0 {
		t.Fatal("缺少测试往来单位")
	}
	wh, err := st.DefaultWarehouse(ctx)
	if err != nil {
		t.Fatalf("读取默认仓库失败: %v", err)
	}
	pid := products[0].ID
	purchase, err := st.SavePurchase(ctx, 0, store.PurchaseInput{
		SupplierID: suppliers[0].ID, WarehouseID: wh.ID, OrderDate: model.Today(),
		Items: []store.OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, purchase, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	sale, err := st.SaveSales(ctx, 0, store.SalesInput{
		CustomerID: customers[0].ID, WarehouseID: wh.ID, OrderDate: model.Today(),
		Items: []store.OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(5), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, sale, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	// 管理员先确认走势确有数据，避免测试因“无数据”而假通过。
	c.login("admin", "admin123")
	adminTrend := decode[[]map[string]any](t, c.do("GET", "/dashboard/trend", nil, 0))
	hasData := false
	for _, p := range adminTrend {
		if p["sales"].(float64) > 0 || p["purchase"].(float64) > 0 {
			hasData = true
			break
		}
	}
	if !hasData {
		t.Fatalf("管理员走势应包含刚创建的单据数据")
	}

	role := decode[map[string]any](t, c.do("POST", "/roles", map[string]any{
		"code": "req8_dashboard_only", "name": "REQ8 仅看板",
		"perms": []string{"dashboard:view"},
	}, 0))
	c.do("POST", "/users", map[string]any{
		"username": "req8_dashboard_user", "password": "abc123456", "name": "REQ8 仅看板",
		"role_id": role["id"], "status": 1,
	}, 0)

	c.login("req8_dashboard_user", "abc123456")
	trend := decode[[]map[string]any](t, c.do("GET", "/dashboard/trend", nil, 0))
	if len(trend) != 0 {
		t.Fatalf("无销售/采购权限时走势应为空，实际返回 %d 个数据点（存在越权）", len(trend))
	}
}

// 有销售权限但没有采购权限时，走势只能保留销售/毛利，采购必须脱敏。
func TestP1DashboardTrendSalesOnlyMasking(t *testing.T) {
	c, st := newTestServer(t)
	ctx := context.Background()
	user := store.OpUser{ID: 1, Name: "测试员"}
	products, _ := st.AllProducts(ctx, "")
	suppliers, _ := st.AllPartners(ctx, "supplier", "")
	customers, _ := st.AllPartners(ctx, "customer", "")
	wh, _ := st.DefaultWarehouse(ctx)
	if len(products) == 0 || len(suppliers) == 0 || len(customers) == 0 {
		t.Fatal("缺少测试基础资料")
	}
	purchase, err := st.SavePurchase(ctx, 0, store.PurchaseInput{
		SupplierID: suppliers[0].ID, WarehouseID: wh.ID, OrderDate: model.Today(),
		Items: []store.OrderItemInput{{ProductID: products[0].ID, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, purchase, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	sale, err := st.SaveSales(ctx, 0, store.SalesInput{
		CustomerID: customers[0].ID, WarehouseID: wh.ID, OrderDate: model.Today(),
		Items: []store.OrderItemInput{{ProductID: products[0].ID, Qty: model.QtyFrom(5), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, sale, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	c.login("admin", "admin123")
	role := decode[map[string]any](t, c.do("POST", "/roles", map[string]any{
		"code": "req8_sales_only", "name": "REQ8 仅销售",
		"perms": []string{"dashboard:view", "sales:view"},
	}, 0))
	c.do("POST", "/users", map[string]any{
		"username": "req8_sales_user", "password": "abc123456", "name": "REQ8 仅销售",
		"role_id": role["id"], "status": 1,
	}, 0)

	c.login("req8_sales_user", "abc123456")
	trend := decode[[]map[string]any](t, c.do("GET", "/dashboard/trend", nil, 0))
	if len(trend) == 0 {
		t.Fatalf("有销售权限时走势不应为空")
	}
	last := trend[len(trend)-1]
	if last["sales"].(float64) <= 0 {
		t.Fatalf("有销售权限时应能看到销售数据，实际 %v", last["sales"])
	}
	for _, p := range trend {
		if p["purchase"].(float64) != 0 {
			t.Fatalf("无采购权限时采购数据必须脱敏，实际 %v", p["purchase"])
		}
	}
}

// meta 只应在管理员仍使用默认口令时提示初始账号。
func TestMetaDefaultLoginFlag(t *testing.T) {
	c, _ := newTestServer(t)

	meta := decode[map[string]any](t, c.do("GET", "/meta", nil, 0))
	flag, ok := meta["default_login"].(bool)
	if !ok || !flag {
		t.Fatalf("初始状态 default_login 应为 true，实际 %v", meta["default_login"])
	}

	c.login("admin", "admin123")
	c.do("POST", "/auth/password", map[string]string{
		"old_password": "admin123", "new_password": "newpass123",
	}, 0)

	meta = decode[map[string]any](t, c.do("GET", "/meta", nil, 0))
	flag, ok = meta["default_login"].(bool)
	if !ok || flag {
		t.Fatalf("修改管理员口令后 default_login 应为 false，实际 %v", meta["default_login"])
	}
}
