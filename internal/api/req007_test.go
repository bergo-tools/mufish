package api

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"testing"

	"mufish/internal/model"
)

// P0-2：通过用户管理接口修改口令后，目标用户的旧会话必须立即失效。
func TestHTTPUpdateUserPasswordInvalidatesOldSession(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	uid := int64(decode[map[string]any](t, c.do("POST", "/users", map[string]any{
		"username": "session_user", "password": "secret1", "name": "会话测试",
		"status": 1,
	}, 0))["id"].(float64))

	jar, _ := cookiejar.New(nil)
	other := &testClient{t: t, base: c.base, client: &http.Client{Jar: jar}}
	other.login("session_user", "secret1")
	other.do("GET", "/auth/me", nil, 0)

	c.do("PUT", fmt.Sprintf("/users/%d", uid), map[string]any{
		"username": "session_user", "name": "会话测试", "status": 1, "password": "newsecret1",
	}, 0)

	// 旧 Cookie 必须被服务端拒绝。
	other.do("GET", "/auth/me", nil, http.StatusUnauthorized)
	// 新口令可以重新登录。
	other.login("session_user", "newsecret1")
	other.do("GET", "/auth/me", nil, 0)
}

// P1-1：销售优惠超过明细金额时 API 返回 400，且不产生单据。
func TestHTTPSalesDiscountOverTotalRejected(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	pid := int64(decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "R7-P1", "name": "折扣测试商品", "unit": "个",
		"purchase_price": 5, "sale_price": 10, "status": 1,
	}, 0))["id"].(float64))
	cid := int64(decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "R7-C1", "name": "折扣测试客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64))

	payload := map[string]any{
		"customer_id": cid, "warehouse_id": 0, "order_date": model.Today(),
		"discount_amount": 11,
		"items":           []map[string]any{{"product_id": pid, "qty": 1, "price": 10}},
	}
	c.do("POST", "/sales", payload, http.StatusBadRequest)

	list := decode[map[string]any](t, c.do("GET", "/sales?size=10", nil, 0))
	if int(list["total"].(float64)) != 0 {
		t.Fatalf("保存失败后不应留下销售单，实际 total=%v", list["total"])
	}
}

// P1-3 / P1-4：冲抵可不选账户；资金流水关键字过滤生效。
func TestHTTPOffsetPaymentAndCashFlowKeyword(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	cid := int64(decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "R7-C2", "name": "冲抵测试客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64))

	// 非冲抵方式必须选择账户。
	c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": cid, "account_id": 0, "pay_date": model.Today(),
		"amount": 10, "method": "bank",
	}, http.StatusBadRequest)

	// 冲抵方式不需要账户。
	pid := int64(decode[map[string]any](t, c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": cid, "account_id": 0, "pay_date": model.Today(),
		"amount": 50, "method": "offset", "remark": "往来冲抵",
	}, 0))["id"].(float64))
	c.do("POST", fmt.Sprintf("/payments/%d/confirm", pid), nil, 0)
	p := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/payments/%d", pid), nil, 0))
	if p["account_id"].(float64) != 0 || p["account_name"].(string) != "" {
		t.Fatalf("冲抵单据不应关联账户，实际 %v", p)
	}

	// 银行收款进入资金流水，并可按单据号搜索。
	accounts := decode[[]map[string]any](t, c.do("GET", "/accounts/options", nil, 0))
	if len(accounts) == 0 {
		t.Fatalf("缺少资金账户")
	}
	bankID := int64(decode[map[string]any](t, c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": cid, "account_id": accounts[0]["id"],
		"pay_date": model.Today(), "amount": 20, "method": "bank",
	}, 0))["id"].(float64))
	c.do("POST", fmt.Sprintf("/payments/%d/confirm", bankID), nil, 0)
	bank := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/payments/%d", bankID), nil, 0))
	no := bank["payment_no"].(string)

	rows := decode[[]map[string]any](t, c.do("GET", "/reports/cash-flow?keyword="+no, nil, 0))
	if len(rows) != 1 {
		t.Fatalf("按单据号搜索资金流水应返回 1 条，实际 %d 条", len(rows))
	}
	empty := decode[[]map[string]any](t, c.do("GET", "/reports/cash-flow?keyword=no-such-payment", nil, 0))
	if len(empty) != 0 {
		t.Fatalf("无匹配关键字应返回 0 条，实际 %d 条", len(empty))
	}
}
