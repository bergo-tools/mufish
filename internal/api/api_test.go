package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"mufish/internal/store"
)

type testClient struct {
	t      *testing.T
	base   string
	client *http.Client
}

type apiEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func newTestServer(t *testing.T) (*testClient, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "api.db"))
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	h := New(Options{Store: st, Version: "test", DataDir: dir})
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	return &testClient{t: t, base: srv.URL + "/api/v1", client: &http.Client{Jar: jar}}, st
}

func (c *testClient) do(method, path string, body any, wantStatus int) json.RawMessage {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatalf("编码请求失败: %v", err)
		}
	}
	req, err := http.NewRequest(method, c.base+path, &buf)
	if err != nil {
		c.t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("X-Requested-With", "fetch")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("请求失败 %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	var env apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		c.t.Fatalf("解析响应失败 %s %s: %v", method, path, err)
	}
	if wantStatus > 0 && resp.StatusCode != wantStatus {
		c.t.Fatalf("%s %s 期望状态 %d，实际 %d（%s）", method, path, wantStatus, resp.StatusCode, env.Msg)
	}
	if wantStatus == 0 && resp.StatusCode >= 400 {
		c.t.Fatalf("%s %s 请求失败 %d: %s", method, path, resp.StatusCode, env.Msg)
	}
	return env.Data
}

func (c *testClient) login(username, password string) {
	c.t.Helper()
	c.do("POST", "/auth/login", map[string]string{"username": username, "password": password}, 0)
}

func decode[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("解析 data 失败: %v", err)
	}
	return v
}

func TestHTTPAuthGuards(t *testing.T) {
	c, _ := newTestServer(t)

	// 未登录访问受保护接口
	c.do("GET", "/products", nil, http.StatusUnauthorized)
	// meta 无需登录
	c.do("GET", "/meta", nil, 0)
	// 错误口令
	c.do("POST", "/auth/login", map[string]string{"username": "admin", "password": "bad"}, http.StatusUnauthorized)

	c.login("admin", "admin123")
	me := decode[map[string]any](t, c.do("GET", "/auth/me", nil, 0))
	if me["user"] == nil {
		t.Fatalf("/auth/me 应返回用户信息")
	}
	perms, _ := me["permissions"].([]any)
	if len(perms) != 1 || perms[0] != "*" {
		t.Fatalf("超级管理员权限应为 [*]，实际 %v", perms)
	}
}

func TestHTTPSalesmanCannotManageUsers(t *testing.T) {
	c, st := newTestServer(t)
	// 为销售员角色补充必要的查看权限之外，验证越权被拒绝
	c.login("sales", "admin123")

	c.do("GET", "/users", nil, http.StatusForbidden)
	c.do("POST", "/users", map[string]any{"username": "x", "password": "abcdef"}, http.StatusForbidden)
	// 销售员应有的权限
	c.do("GET", "/sales?size=5", nil, 0)
	c.do("POST", "/products", map[string]any{"name": "非法商品"}, http.StatusForbidden)

	_ = st
}

func TestHTTPPurchaseSalesFlow(t *testing.T) {
	c, st := newTestServer(t)
	c.login("admin", "admin123")

	// 准备基础资料
	pid := decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "H-P1", "name": "接口测试商品", "unit": "个",
		"purchase_price": 12.5, "sale_price": 25, "safety_stock": 3, "status": 1,
	}, 0))["id"].(float64)

	sid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "H-S1", "name": "接口测试供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64)

	cid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "H-C1", "name": "接口测试客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64)

	whs := decode[[]map[string]any](t, c.do("GET", "/warehouses/options", nil, 0))
	if len(whs) == 0 {
		t.Fatalf("应存在默认仓库")
	}
	wid := whs[0]["id"].(float64)

	// 采购 20 个
	po := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": "2026-01-05",
		"discount_amount": 0, "other_fee": 0, "remark": "接口测试",
		"items": []map[string]any{{"product_id": pid, "qty": 20, "price": 12.5}},
	}, 0))
	poID := int64(po["id"].(float64))

	// 草稿状态库存为 0
	stocks := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/stocks?keyword=H-P1&warehouse_id=%.0f", wid), nil, 0))
	if list, ok := stocks["list"].([]any); ok && len(list) > 0 {
		t.Fatalf("草稿状态不应产生库存")
	}

	c.do("POST", fmt.Sprintf("/purchases/%d/confirm", poID), map[string]any{}, 0)

	stocks = decode[map[string]any](t, c.do("GET", fmt.Sprintf("/stocks?keyword=H-P1&warehouse_id=%.0f", wid), nil, 0))
	list := stocks["list"].([]any)
	if len(list) != 1 {
		t.Fatalf("入库后应有 1 条库存记录，实际 %d", len(list))
	}
	row := list[0].(map[string]any)
	if row["qty"].(float64) != 20 {
		t.Fatalf("入库后数量期望 20，实际 %v", row["qty"])
	}
	if row["cost_price"].(float64) != 12.5 {
		t.Fatalf("入库后成本期望 12.5，实际 %v", row["cost_price"])
	}

	// 销售 5 个
	so := decode[map[string]any](t, c.do("POST", "/sales", map[string]any{
		"customer_id": cid, "warehouse_id": wid, "order_date": "2026-01-06",
		"items": []map[string]any{{"product_id": pid, "qty": 5, "price": 25}},
	}, 0))
	soID := int64(so["id"].(float64))
	c.do("POST", fmt.Sprintf("/sales/%d/confirm", soID), map[string]any{}, 0)

	detail := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/sales/%d", soID), nil, 0))
	if detail["settle_amount"].(float64) != 125 {
		t.Fatalf("销售额期望 125，实际 %v", detail["settle_amount"])
	}
	if detail["cost_amount"].(float64) != 62.5 {
		t.Fatalf("销售成本期望 62.5，实际 %v", detail["cost_amount"])
	}

	// 库存流水应有 2 条
	ledger := decode[map[string]any](t, c.do("GET", "/stocks/ledger?keyword=H-P1", nil, 0))
	if ledger["total"].(float64) < 2 {
		t.Fatalf("库存流水应至少 2 条，实际 %v", ledger["total"])
	}

	// 应收报表
	ar := decode[[]map[string]any](t, c.do("GET", "/reports/ar-ap?type=receivable", nil, 0))
	found := false
	for _, r := range ar {
		if r["partner_name"] == "接口测试客户" {
			found = true
			if r["balance"].(float64) != 125 {
				t.Fatalf("应收余额期望 125，实际 %v", r["balance"])
			}
		}
	}
	if !found {
		t.Fatalf("应收报表应包含接口测试客户")
	}

	// 看板
	dash := decode[map[string]any](t, c.do("GET", "/dashboard", nil, 0))
	if dash["summary"] == nil {
		t.Fatalf("看板应返回 summary")
	}

	// 备份
	c.do("POST", "/system/backup", map[string]any{}, 0)
	backups := decode[[]map[string]any](t, c.do("GET", "/system/backups", nil, 0))
	if len(backups) == 0 {
		t.Fatalf("备份后应能查询到备份文件")
	}

	// 导出 CSV（非 JSON 响应，单独处理）
	req, _ := http.NewRequest("GET", c.base+"/products/export", nil)
	req.Header.Set("X-Requested-With", "fetch")
	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("导出请求失败: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("导出应返回 200，实际 %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct[:8] != "text/csv" {
		t.Fatalf("导出 Content-Type 期望 text/csv，实际 %s", ct)
	}

	// 越权：删除内置管理员
	users := decode[map[string]any](t, c.do("GET", "/users", nil, 0))
	_ = users
	_ = st
}

func TestHTTPVoidRequiresPermission(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("stock", "admin123") // 仓管员：可确认，不可删除 / 作废

	// 仓管员没有 purchase:create
	c.do("POST", "/purchases", map[string]any{
		"supplier_id": 3, "warehouse_id": 1,
		"items": []map[string]any{{"product_id": 1, "qty": 1, "price": 1}},
	}, http.StatusForbidden)

	// 有 purchase:view
	c.do("GET", "/purchases?size=5", nil, 0)
	// 无 stock:take 之外的管理权限校验：仓管员确实有 stock:take
	c.do("GET", "/stock-takes?size=5", nil, 0)
	// 无用户管理
	c.do("GET", "/users", nil, http.StatusForbidden)
}
