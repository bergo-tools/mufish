package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"mufish/internal/model"
)

// 调账走独立接口并生成记录；编辑账户不能改余额。
func TestHTTPAccountAdjustmentFlow(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	acc := decode[map[string]any](t, c.do("POST", "/accounts", map[string]any{
		"code": "REQ9-A1", "name": "调账测试账户", "type": "cash",
		"balance": 100, "status": 1,
	}, 0))
	accID := int64(acc["id"].(float64))
	accPath := fmt.Sprintf("/accounts/%d", accID)

	// 编辑账户即使带 balance 也不能覆盖余额。
	c.do("PUT", accPath, map[string]any{
		"code": "REQ9-A1", "name": "调账测试账户（改）", "type": "cash",
		"balance": 999, "status": 1,
	}, 0)
	accounts := decode[[]map[string]any](t, c.do("GET", "/accounts", nil, 0))
	var balance float64
	for _, a := range accounts {
		if int64(a["id"].(float64)) == accID {
			balance = a["balance"].(float64)
		}
	}
	if balance != 100 {
		t.Fatalf("编辑账户不应改变余额，期望 100，实际 %v", balance)
	}

	adj := decode[map[string]any](t, c.do("POST", accPath+"/adjust", map[string]any{
		"balance": 250, "reason": "期初余额修正",
	}, 0))
	if adj["before_balance"].(float64) != 100 || adj["after_balance"].(float64) != 250 ||
		adj["amount"].(float64) != 150 || adj["reason"] != "期初余额修正" {
		t.Fatalf("调账返回不正确: %v", adj)
	}

	list := decode[[]map[string]any](t, c.do("GET", accPath+"/adjustments", nil, 0))
	if len(list) != 1 || list[0]["reason"] != "期初余额修正" {
		t.Fatalf("调账记录列表不正确: %v", list)
	}
	logs := decode[map[string]any](t, c.do("GET", "/logs?module=资金账户&keyword=余额&size=10", nil, 0))
	var detail string
	for _, item := range logs["list"].([]any) {
		detail += item.(map[string]any)["detail"].(string)
	}
	if !strings.Contains(detail, "余额 100.00 → 250.00") || !strings.Contains(detail, "期初余额修正") {
		t.Fatalf("调账审计缺少旧值/新值或原因: %s", detail)
	}

	// 非法调账不改变余额。
	c.do("POST", accPath+"/adjust", map[string]any{"balance": 250, "reason": "相同"}, http.StatusBadRequest)
	c.do("POST", accPath+"/adjust", map[string]any{"balance": 300, "reason": " "}, http.StatusBadRequest)
	c.do("POST", accPath+"/adjust", map[string]any{"reason": "缺少余额"}, http.StatusBadRequest)
	accounts = decode[[]map[string]any](t, c.do("GET", "/accounts", nil, 0))
	for _, a := range accounts {
		if int64(a["id"].(float64)) == accID && a["balance"].(float64) != 250 {
			t.Fatalf("失败调账不应改变余额，实际 %v", a["balance"])
		}
	}

	// 无 account:manage 权限不能调账。
	c.login("sales", "admin123")
	c.do("POST", accPath+"/adjust", map[string]any{"balance": 260, "reason": "越权"}, http.StatusForbidden)

	// 有调账记录的账户不能被删除。
	c.login("admin", "admin123")
	c.do("POST", accPath+"/adjust", map[string]any{"balance": 0, "reason": "清零"}, 0)
	c.do("DELETE", accPath, nil, http.StatusConflict)
}

// 商品选项按仓库返回可用库存。
func TestHTTPProductOptionsWarehouseStock(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	pid := int64(decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "REQ9-P1", "name": "分仓库存接口商品", "unit": "个",
		"purchase_price": 10, "sale_price": 20, "status": 1,
	}, 0))["id"].(float64))
	sid := int64(decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "REQ9-S1", "name": "分仓库存供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64))
	wa := int64(decode[map[string]any](t, c.do("POST", "/warehouses", map[string]any{
		"code": "REQ9-WA", "name": "分仓 A", "status": 1,
	}, 0))["id"].(float64))
	wb := int64(decode[map[string]any](t, c.do("POST", "/warehouses", map[string]any{
		"code": "REQ9-WB", "name": "分仓 B", "status": 1,
	}, 0))["id"].(float64))

	po := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wa, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 7, "price": 10}},
	}, 0))
	c.do("POST", fmt.Sprintf("/purchases/%d/confirm", int64(po["id"].(float64))), map[string]any{}, 0)

	check := func(warehouse int64, want float64) {
		t.Helper()
		rows := decode[[]map[string]any](t, c.do("GET",
			fmt.Sprintf("/products/options?keyword=REQ9-P1&warehouse_id=%d", warehouse), nil, 0))
		if len(rows) != 1 || rows[0]["stock_qty"].(float64) != want {
			t.Fatalf("仓库 %d 可用库存期望 %v，实际 %v", warehouse, want, rows)
		}
	}
	check(wa, 7)
	check(wb, 0)
}
