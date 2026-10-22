package api

import (
	"fmt"
	"net/http"
	"testing"

	"mufish/internal/model"
)

func TestHTTPStockTakeTransferLifecycle(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	pid := decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "LC-P1", "name": "生命周期测试商品", "unit": "个",
		"purchase_price": 10, "sale_price": 20, "status": 1,
	}, 0))["id"].(float64)
	sid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "LC-S1", "name": "生命周期测试供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64)
	wid := decode[[]map[string]any](t, c.do("GET", "/warehouses/options", nil, 0))[0]["id"].(float64)

	purchase := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 10, "price": 10}},
	}, 0))
	purchaseID := int64(purchase["id"].(float64))
	c.do("POST", fmt.Sprintf("/purchases/%d/confirm", purchaseID), map[string]any{}, 0)

	// 草稿盘点：查询、修改、删除。
	take := decode[map[string]any](t, c.do("POST", "/stock-takes", map[string]any{
		"warehouse_id": wid, "take_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "real_qty": 10}},
	}, 0))
	takeID := int64(take["id"].(float64))
	list := decode[map[string]any](t, c.do("GET", "/stock-takes?keyword=PD&size=5", nil, 0))
	if list["total"].(float64) < 1 {
		t.Fatalf("盘点单列表应包含新建盘点单: %v", list)
	}
	detail := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/stock-takes/%d", takeID), nil, 0))
	if len(detail["items"].([]any)) != 1 {
		t.Fatalf("盘点单明细不正确: %v", detail)
	}
	c.do("PUT", fmt.Sprintf("/stock-takes/%d", takeID), map[string]any{
		"warehouse_id": wid, "take_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "real_qty": 9}},
	}, 0)
	c.do("DELETE", fmt.Sprintf("/stock-takes/%d", takeID), nil, 0)

	// 已确认盘点：完成、作废，作废后不可删除。
	take2 := decode[map[string]any](t, c.do("POST", "/stock-takes", map[string]any{
		"warehouse_id": wid, "take_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "real_qty": 9}},
	}, 0))
	take2ID := int64(take2["id"].(float64))
	c.do("POST", fmt.Sprintf("/stock-takes/%d/finish", take2ID), map[string]any{}, 0)
	c.do("POST", fmt.Sprintf("/stock-takes/%d/void", take2ID), map[string]any{}, 0)
	c.do("DELETE", fmt.Sprintf("/stock-takes/%d", take2ID), nil, http.StatusBadRequest)

	w2 := decode[map[string]any](t, c.do("POST", "/warehouses", map[string]any{
		"code": "LC-W2", "name": "生命周期二仓", "status": 1,
	}, 0))["id"].(float64)

	// 草稿调拨：查询、修改、删除。
	transfer := decode[map[string]any](t, c.do("POST", "/transfers", map[string]any{
		"from_warehouse_id": wid, "to_warehouse_id": w2, "transfer_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 5}},
	}, 0))
	transferID := int64(transfer["id"].(float64))
	list = decode[map[string]any](t, c.do("GET", "/transfers?keyword=DB&size=5", nil, 0))
	if list["total"].(float64) < 1 {
		t.Fatalf("调拨单列表应包含新建调拨单: %v", list)
	}
	detail = decode[map[string]any](t, c.do("GET", fmt.Sprintf("/transfers/%d", transferID), nil, 0))
	if len(detail["items"].([]any)) != 1 {
		t.Fatalf("调拨单明细不正确: %v", detail)
	}
	c.do("PUT", fmt.Sprintf("/transfers/%d", transferID), map[string]any{
		"from_warehouse_id": wid, "to_warehouse_id": w2, "transfer_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 5}},
	}, 0)
	c.do("DELETE", fmt.Sprintf("/transfers/%d", transferID), nil, 0)

	// 已确认调拨：作废，作废后不可删除。
	transfer2 := decode[map[string]any](t, c.do("POST", "/transfers", map[string]any{
		"from_warehouse_id": wid, "to_warehouse_id": w2, "transfer_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 5}},
	}, 0))
	transfer2ID := int64(transfer2["id"].(float64))
	c.do("POST", fmt.Sprintf("/transfers/%d/confirm", transfer2ID), map[string]any{}, 0)
	c.do("POST", fmt.Sprintf("/transfers/%d/void", transfer2ID), map[string]any{}, 0)
	c.do("DELETE", fmt.Sprintf("/transfers/%d", transfer2ID), nil, http.StatusBadRequest)
}

func TestHTTPUserRoleLogLifecycle(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	roles := decode[[]map[string]any](t, c.do("GET", "/roles", nil, 0))
	if len(roles) != 7 {
		t.Fatalf("内置角色期望 7 个，实际 %d", len(roles))
	}
	var adminRoleID float64
	for _, r := range roles {
		if r["code"] == "admin" {
			adminRoleID = r["id"].(float64)
		}
	}
	if adminRoleID == 0 {
		t.Fatalf("未找到内置管理员角色")
	}
	c.do("PUT", fmt.Sprintf("/roles/%.0f", adminRoleID), map[string]any{"name": "越权改名"}, http.StatusBadRequest)
	c.do("DELETE", fmt.Sprintf("/roles/%.0f", adminRoleID), nil, http.StatusBadRequest)

	role := decode[map[string]any](t, c.do("POST", "/roles", map[string]any{
		"code": "api_role", "name": "API角色", "perms": []string{"stock:view"},
	}, 0))
	roleID := role["id"].(float64)
	c.do("PUT", fmt.Sprintf("/roles/%.0f", roleID), map[string]any{
		"name": "API角色（改）", "perms": []string{"stock:view", "report:view"},
	}, 0)

	c.do("POST", "/users", map[string]any{
		"username": "short_user", "password": "123", "name": "短口令",
	}, http.StatusBadRequest)
	user := decode[map[string]any](t, c.do("POST", "/users", map[string]any{
		"username": "api_user", "password": "abc123456", "name": "API用户",
		"role_id": roleID, "status": 1,
	}, 0))
	userID := user["id"].(float64)
	users := decode[map[string]any](t, c.do("GET", "/users?keyword=api_user&size=10", nil, 0))
	if users["total"].(float64) != 1 {
		t.Fatalf("用户关键字查询期望 1，实际 %v", users["total"])
	}
	c.do("PUT", fmt.Sprintf("/users/%.0f", userID), map[string]any{
		"username": "api_user", "name": "API用户（改）", "role_id": roleID, "status": 1,
	}, 0)
	c.do("POST", fmt.Sprintf("/users/%.0f/reset-password", userID), map[string]any{
		"password": "newpass123",
	}, 0)
	c.do("DELETE", fmt.Sprintf("/users/%.0f", userID), nil, 0)
	c.do("DELETE", fmt.Sprintf("/roles/%.0f", roleID), nil, 0)

	me := decode[map[string]any](t, c.do("GET", "/auth/me", nil, 0))
	adminID := me["user"].(map[string]any)["id"].(float64)
	c.do("DELETE", fmt.Sprintf("/users/%.0f", adminID), nil, http.StatusBadRequest)

	logs := decode[map[string]any](t, c.do("GET", "/logs?module=用户管理&size=10", nil, 0))
	if logs["total"].(float64) == 0 {
		t.Fatalf("用户管理日志不应为空")
	}
	mods := decode[[]any](t, c.do("GET", "/logs/modules", nil, 0))
	if len(mods) == 0 {
		t.Fatalf("日志模块列表不应为空")
	}
}

func TestHTTPPurchaseSalesVoidAndDelete(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	pid := decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "VOID-P1", "name": "作废测试商品", "unit": "个",
		"purchase_price": 10, "sale_price": 20, "status": 1,
	}, 0))["id"].(float64)
	sid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "VOID-S1", "name": "作废测试供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64)
	cid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "VOID-C1", "name": "作废测试客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64)
	wid := decode[[]map[string]any](t, c.do("GET", "/warehouses/options", nil, 0))[0]["id"].(float64)

	// 草稿采购可删除。
	draft := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 1, "price": 10}},
	}, 0))
	draftID := int64(draft["id"].(float64))
	c.do("DELETE", fmt.Sprintf("/purchases/%d", draftID), nil, 0)

	// 已确认采购作废后不可删除，并释放库存用于后续销售。
	purchase := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 10, "price": 10}},
	}, 0))
	purchaseID := int64(purchase["id"].(float64))
	c.do("POST", fmt.Sprintf("/purchases/%d/confirm", purchaseID), map[string]any{}, 0)
	c.do("POST", fmt.Sprintf("/purchases/%d/void", purchaseID), map[string]any{}, 0)
	c.do("DELETE", fmt.Sprintf("/purchases/%d", purchaseID), nil, http.StatusBadRequest)

	purchase2 := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 10, "price": 10}},
	}, 0))
	purchase2ID := int64(purchase2["id"].(float64))
	c.do("POST", fmt.Sprintf("/purchases/%d/confirm", purchase2ID), map[string]any{}, 0)

	// 关键字过滤应正常（采购和销售使用不同的往来单位字段）。
	purchases := decode[map[string]any](t, c.do("GET", "/purchases?keyword=作废测试供应商&size=10", nil, 0))
	if purchases["total"].(float64) == 0 {
		t.Fatalf("采购单关键字过滤不应为空")
	}

	draftSale := decode[map[string]any](t, c.do("POST", "/sales", map[string]any{
		"customer_id": cid, "warehouse_id": wid, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 1, "price": 20}},
	}, 0))
	draftSaleID := int64(draftSale["id"].(float64))
	c.do("DELETE", fmt.Sprintf("/sales/%d", draftSaleID), nil, 0)

	sale := decode[map[string]any](t, c.do("POST", "/sales", map[string]any{
		"customer_id": cid, "warehouse_id": wid, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 5, "price": 20}},
	}, 0))
	saleID := int64(sale["id"].(float64))
	c.do("POST", fmt.Sprintf("/sales/%d/confirm", saleID), map[string]any{}, 0)
	c.do("POST", fmt.Sprintf("/sales/%d/void", saleID), map[string]any{}, 0)
	c.do("DELETE", fmt.Sprintf("/sales/%d", saleID), nil, http.StatusBadRequest)

	sales := decode[map[string]any](t, c.do("GET", "/sales?keyword=作废测试客户&size=10", nil, 0))
	if sales["total"].(float64) == 0 {
		t.Fatalf("销售单关键字过滤不应为空")
	}
}

func TestHTTPPaymentListGetDelete(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	cid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "PL-C1", "name": "付款列表客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64)
	accounts := decode[[]map[string]any](t, c.do("GET", "/accounts/options", nil, 0))
	accID := accounts[0]["id"].(float64)

	payment := decode[map[string]any](t, c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": cid, "account_id": accID,
		"pay_date": model.Today(), "amount": 10, "method": "cash",
	}, 0))
	paymentID := int64(payment["id"].(float64))

	detail := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/payments/%d", paymentID), nil, 0))
	if detail["amount"].(float64) != 10 {
		t.Fatalf("收付款详情金额不正确: %v", detail)
	}
	list := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/payments?type=receipt&partner_id=%.0f&size=10", cid), nil, 0))
	if list["total"].(float64) != 1 {
		t.Fatalf("收付款列表期望 1，实际 %v", list["total"])
	}
	c.do("DELETE", fmt.Sprintf("/payments/%d", paymentID), nil, 0)
	c.do("GET", fmt.Sprintf("/payments/%d", paymentID), nil, http.StatusNotFound)
}
