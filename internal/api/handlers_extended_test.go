package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"testing"

	"mufish/internal/model"
)

func (c *testClient) rawGet(path string) *http.Response {
	c.t.Helper()
	req, err := http.NewRequest("GET", c.base+path, nil)
	if err != nil {
		c.t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("X-Requested-With", "fetch")
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatalf("请求失败 GET %s: %v", path, err)
	}
	return resp
}

func TestHTTPAuthLifecycle(t *testing.T) {
	c, _ := newTestServer(t)

	meta := decode[map[string]any](t, c.do("GET", "/meta", nil, 0))
	if meta["logged_in"].(bool) {
		t.Fatalf("未登录时 meta.logged_in 应为 false")
	}

	c.do("POST", "/auth/login", map[string]string{"username": "admin", "password": "bad"}, http.StatusUnauthorized)
	data := decode[map[string]any](t, c.do("POST", "/auth/login", map[string]string{
		"username": "admin", "password": "admin123",
	}, 0))
	perms := data["permissions"].([]any)
	if len(perms) != 1 || perms[0] != "*" {
		t.Fatalf("超级管理员权限应为 [*]，实际 %v", perms)
	}

	me := decode[map[string]any](t, c.do("GET", "/auth/me", nil, 0))
	u := me["user"].(map[string]any)
	if u["username"] != "admin" {
		t.Fatalf("/auth/me 用户不正确: %v", u)
	}

	c.do("POST", "/auth/password", map[string]string{
		"old_password": "bad", "new_password": "newpass123",
	}, http.StatusBadRequest)
	c.do("POST", "/auth/password", map[string]string{
		"old_password": "admin123", "new_password": "newpass123",
	}, 0)

	c.do("POST", "/auth/logout", map[string]any{}, 0)
	c.do("GET", "/products", nil, http.StatusUnauthorized)
	c.do("POST", "/auth/login", map[string]string{"username": "admin", "password": "newpass123"}, 0)
	c.do("GET", "/products", nil, 0)
}

func TestHTTPCRUDAndErrors(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	c.do("POST", "/categories", map[string]any{"name": ""}, http.StatusBadRequest)
	root := decode[map[string]any](t, c.do("POST", "/categories", map[string]any{"name": "API根分类"}, 0))
	rootID := int64(root["id"].(float64))
	child := decode[map[string]any](t, c.do("POST", "/categories", map[string]any{
		"name": "API子分类", "parent_id": rootID,
	}, 0))
	childID := int64(child["id"].(float64))

	tree := decode[[]map[string]any](t, c.do("GET", "/categories", nil, 0))
	var rootNode map[string]any
	for _, n := range tree {
		if int64(n["id"].(float64)) == rootID {
			rootNode = n
		}
	}
	if rootNode == nil {
		t.Fatalf("分类树中应包含新根分类")
	}
	children, _ := rootNode["children"].([]any)
	if len(children) != 1 || int64(children[0].(map[string]any)["id"].(float64)) != childID {
		t.Fatalf("分类树子节点不正确: %+v", rootNode)
	}

	product := decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "API-P1", "name": "API测试商品", "unit": "个",
		"category_id": childID, "purchase_price": 10, "sale_price": 20, "status": 1,
	}, 0))
	pid := int64(product["id"].(float64))

	c.do("POST", "/products", map[string]any{"code": "API-P1", "name": "重复编码"}, http.StatusConflict)
	c.do("POST", "/products", map[string]any{"name": ""}, http.StatusBadRequest)
	c.do("GET", "/products/999999", nil, http.StatusNotFound)
	c.do("PUT", "/products/999999", map[string]any{"code": "API-NONE", "name": "不存在"}, http.StatusNotFound)

	page := decode[map[string]any](t, c.do("GET", "/products?size=2&page=1", nil, 0))
	if len(page["list"].([]any)) != 2 {
		t.Fatalf("分页 size=2 应返回 2 条，实际 %d", len(page["list"].([]any)))
	}
	if page["total"].(float64) < 2 {
		t.Fatalf("商品总数应至少为 2，实际 %v", page["total"])
	}

	partner := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "API-C1", "name": "API测试客户", "type": "customer", "status": 1,
	}, 0))
	c.do("PUT", fmt.Sprintf("/partners/%v", partner["id"]), map[string]any{
		"code": "API-C1", "name": "API测试客户（改）", "type": "customer", "status": 1,
	}, 0)
	c.do("GET", "/partners/999999", nil, http.StatusNotFound)
	c.do("DELETE", fmt.Sprintf("/partners/%v", partner["id"]), nil, 0)
	c.do("DELETE", fmt.Sprintf("/partners/%v", partner["id"]), nil, http.StatusNotFound)

	wh := decode[map[string]any](t, c.do("POST", "/warehouses", map[string]any{
		"code": "API-W1", "name": "API测试仓", "status": 1,
	}, 0))
	c.do("PUT", fmt.Sprintf("/warehouses/%v", wh["id"]), map[string]any{
		"code": "API-W1", "name": "API测试仓（改）", "status": 1,
	}, 0)
	c.do("DELETE", fmt.Sprintf("/warehouses/%v", wh["id"]), nil, 0)

	acc := decode[map[string]any](t, c.do("POST", "/accounts", map[string]any{
		"code": "API-A1", "name": "API测试账户", "type": "cash", "status": 1,
	}, 0))
	c.do("PUT", fmt.Sprintf("/accounts/%v", acc["id"]), map[string]any{
		"code": "API-A1", "name": "API测试账户（改）", "type": "cash", "status": 1,
	}, 0)
	c.do("DELETE", fmt.Sprintf("/accounts/%v", acc["id"]), nil, 0)

	c.do("DELETE", fmt.Sprintf("/products/%d", pid), nil, 0)
	c.do("DELETE", fmt.Sprintf("/products/%d", pid), nil, http.StatusNotFound)
}

func TestHTTPPermissionsAndSettings(t *testing.T) {
	c, _ := newTestServer(t)

	c.login("sales", "admin123")
	c.do("GET", "/settings", nil, http.StatusForbidden)
	c.do("PUT", "/settings", map[string]any{"company_name": "越权"}, http.StatusForbidden)
	c.do("GET", "/permissions", nil, http.StatusForbidden)
	c.do("GET", "/users", nil, http.StatusForbidden)

	c.login("admin", "admin123")
	permData := decode[map[string]any](t, c.do("GET", "/permissions", nil, 0))
	if len(permData["catalog"].([]any)) != 12 {
		t.Fatalf("权限分组期望 12，实际 %d", len(permData["catalog"].([]any)))
	}
	if len(permData["all"].([]any)) != 46 {
		t.Fatalf("权限点期望 46，实际 %d", len(permData["all"].([]any)))
	}

	c.do("PUT", "/settings", map[string]any{
		"company_name": "API测试公司", "session_hours": "8",
	}, 0)
	settings := decode[map[string]any](t, c.do("GET", "/settings", nil, 0))
	if settings["company_name"] != "API测试公司" || settings["session_hours"] != "8" {
		t.Fatalf("参数保存不正确: %v", settings)
	}
	meta := decode[map[string]any](t, c.do("GET", "/meta", nil, 0))
	if meta["company_name"] != "API测试公司" {
		t.Fatalf("meta 应返回最新公司名称，实际 %v", meta["company_name"])
	}
}

func TestHTTPPaymentFlow(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	pid := decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "PAY-P1", "name": "收付款测试商品", "unit": "个",
		"purchase_price": 10, "sale_price": 20, "status": 1,
	}, 0))["id"].(float64)
	sid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "PAY-S1", "name": "收付款测试供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64)
	cid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "PAY-C1", "name": "收付款测试客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64)
	whs := decode[[]map[string]any](t, c.do("GET", "/warehouses/options", nil, 0))
	wid := whs[0]["id"].(float64)

	purchase := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": "2026-01-05",
		"items": []map[string]any{{"product_id": pid, "qty": 10, "price": 10}},
	}, 0))
	purchaseID := int64(purchase["id"].(float64))
	c.do("POST", fmt.Sprintf("/purchases/%d/confirm", purchaseID), map[string]any{}, 0)

	sale := decode[map[string]any](t, c.do("POST", "/sales", map[string]any{
		"customer_id": cid, "warehouse_id": wid, "order_date": "2026-01-06",
		"items": []map[string]any{{"product_id": pid, "qty": 5, "price": 20}},
	}, 0))
	saleID := int64(sale["id"].(float64))
	c.do("POST", fmt.Sprintf("/sales/%d/confirm", saleID), map[string]any{}, 0)

	unpaid := decode[[]map[string]any](t, c.do("GET", fmt.Sprintf("/payments/unpaid-orders?type=sales&partner_id=%.0f", cid), nil, 0))
	if len(unpaid) != 1 || unpaid[0]["balance"].(float64) != 100 {
		t.Fatalf("未结销售单不正确: %v", unpaid)
	}
	accounts := decode[[]map[string]any](t, c.do("GET", "/accounts/options", nil, 0))
	if len(accounts) == 0 {
		t.Fatalf("应存在资金账户")
	}
	accID := accounts[0]["id"].(float64)

	// 收款单不能关联采购单。
	c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": cid, "account_id": accID,
		"pay_date": "2026-01-06", "amount": 10, "method": "bank",
		"order_type": "purchase", "order_id": purchaseID,
	}, http.StatusBadRequest)

	receipt := decode[map[string]any](t, c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": cid, "account_id": accID,
		"pay_date": "2026-01-06", "amount": 40, "method": "bank",
		"order_type": "sales", "order_id": saleID,
	}, 0))
	receiptID := int64(receipt["id"].(float64))
	c.do("POST", fmt.Sprintf("/payments/%d/confirm", receiptID), map[string]any{}, 0)

	saleDetail := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/sales/%d", saleID), nil, 0))
	if saleDetail["received_amount"].(float64) != 40 {
		t.Fatalf("收款后已收期望 40，实际 %v", saleDetail["received_amount"])
	}
	ar := decode[[]map[string]any](t, c.do("GET", "/reports/ar-ap?type=receivable", nil, 0))
	var balance float64 = -1
	for _, r := range ar {
		if r["partner_name"] == "收付款测试客户" {
			balance = r["balance"].(float64)
		}
	}
	if balance != 60 {
		t.Fatalf("收款后应收余额期望 60，实际 %v", balance)
	}

	c.do("POST", fmt.Sprintf("/payments/%d/void", receiptID), map[string]any{}, 0)
	saleDetail = decode[map[string]any](t, c.do("GET", fmt.Sprintf("/sales/%d", saleID), nil, 0))
	if saleDetail["received_amount"].(float64) != 0 {
		t.Fatalf("作废收款后已收期望 0，实际 %v", saleDetail["received_amount"])
	}

	payment := decode[map[string]any](t, c.do("POST", "/payments", map[string]any{
		"type": "payment", "partner_id": sid, "account_id": accID,
		"pay_date": "2026-01-06", "amount": 50, "method": "bank",
		"order_type": "purchase", "order_id": purchaseID,
	}, 0))
	paymentID := int64(payment["id"].(float64))
	c.do("POST", fmt.Sprintf("/payments/%d/confirm", paymentID), map[string]any{}, 0)
	purchaseDetail := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/purchases/%d", purchaseID), nil, 0))
	if purchaseDetail["paid_amount"].(float64) != 50 {
		t.Fatalf("付款后已付期望 50，实际 %v", purchaseDetail["paid_amount"])
	}
}

func TestHTTPLedgerExportIsNotTruncatedAt500(t *testing.T) {
	c, st := newTestServer(t)
	c.login("admin", "admin123")

	products := decode[[]map[string]any](t, c.do("GET", "/products/options", nil, 0))
	warehouses := decode[[]map[string]any](t, c.do("GET", "/warehouses/options", nil, 0))
	if len(products) == 0 || len(warehouses) == 0 {
		t.Fatalf("缺少商品或仓库")
	}
	pid := products[0]["id"].(float64)
	wid := warehouses[0]["id"].(float64)
	at := model.Today() + " 12:00:00"
	for i := 0; i < 501; i++ {
		if _, err := st.DB().Exec(
			`INSERT INTO stock_moves(warehouse_id, product_id, biz_type, biz_id, biz_no, direction,
			   qty, price, amount, balance_after, remark, created_by, created_by_name, created_at)
			 VALUES(?,?,'test',1,'TEST',1,1,10000,10000,1,'',1,'测试',?)`, wid, pid, at); err != nil {
			t.Fatalf("插入库存流水失败: %v", err)
		}
	}

	resp := c.rawGet("/stocks/ledger/export?biz_type=test")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("导出库存流水期望 200，实际 %d", resp.StatusCode)
	}
	if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("导出库存流水应带 UTF-8 BOM")
	}
	// 表头 + 501 行数据，每行以换行符结尾。
	if got := bytes.Count(body, []byte("\n")); got != 502 {
		t.Fatalf("导出库存流水应包含 501 行数据，实际换行数 %d", got)
	}
}

func TestHTTPStockAndReportEndpoints(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	pid := decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "REP-P1", "name": "报表接口商品", "unit": "个",
		"purchase_price": 10, "sale_price": 20, "safety_stock": 10, "status": 1,
	}, 0))["id"].(float64)
	sid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "REP-S1", "name": "报表接口供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64)
	cid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "REP-C1", "name": "报表接口客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64)
	wid := decode[map[string]any](t, c.do("POST", "/warehouses", map[string]any{
		"code": "REP-W1", "name": "报表接口仓", "status": 1,
	}, 0))["id"].(float64)

	day := model.Today()
	purchase := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": day,
		"items": []map[string]any{{"product_id": pid, "qty": 10, "price": 10}},
	}, 0))
	purchaseID := int64(purchase["id"].(float64))
	c.do("POST", fmt.Sprintf("/purchases/%d/confirm", purchaseID), map[string]any{}, 0)
	sale := decode[map[string]any](t, c.do("POST", "/sales", map[string]any{
		"customer_id": cid, "warehouse_id": wid, "order_date": day,
		"items": []map[string]any{{"product_id": pid, "qty": 3, "price": 20}},
	}, 0))
	saleID := int64(sale["id"].(float64))
	c.do("POST", fmt.Sprintf("/sales/%d/confirm", saleID), map[string]any{}, 0)

	stocks := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/stocks?warehouse_id=%.0f&keyword=REP-P1", wid), nil, 0))
	if stocks["total"].(float64) != 1 {
		t.Fatalf("库存查询期望 1 条，实际 %v", stocks["total"])
	}
	stockRow := stocks["list"].([]any)[0].(map[string]any)
	if stockRow["qty"].(float64) != 7 {
		t.Fatalf("库存数量期望 7，实际 %v", stockRow["qty"])
	}

	summary := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/stocks/summary?warehouse_id=%.0f", wid), nil, 0))
	if summary["kinds"].(float64) != 1 || summary["qty"].(float64) != 7 || summary["amount"].(float64) != 70 {
		t.Fatalf("库存汇总不正确: %v", summary)
	}

	alerts := decode[[]map[string]any](t, c.do("GET", fmt.Sprintf("/stocks/alerts?warehouse_id=%.0f", wid), nil, 0))
	var alertRow map[string]any
	for _, a := range alerts {
		if a["product_code"] == "REP-P1" {
			alertRow = a
		}
	}
	if alertRow == nil || alertRow["qty"].(float64) != 7 {
		t.Fatalf("库存预警不正确: %v", alerts)
	}

	ledger := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/stocks/ledger?warehouse_id=%.0f", wid), nil, 0))
	if ledger["total"].(float64) != 2 {
		t.Fatalf("库存流水期望 2 条，实际 %v", ledger["total"])
	}

	dash := decode[map[string]any](t, c.do("GET", "/dashboard", nil, 0))
	if dash["summary"] == nil || dash["trend"] == nil {
		t.Fatalf("看板响应缺少 summary/trend: %v", dash)
	}

	inout := decode[[]map[string]any](t, c.do("GET", "/reports/inout?from="+day+"&to="+day+"&keyword=REP-P1", nil, 0))
	if len(inout) != 1 || inout[0]["in_qty"].(float64) != 10 || inout[0]["out_qty"].(float64) != 3 || inout[0]["end_qty"].(float64) != 7 {
		t.Fatalf("进销存汇总不正确: %v", inout)
	}
	if inout[0]["end_amount"].(float64) != 70 {
		t.Fatalf("进销存期末金额期望 70，实际 %v", inout[0]["end_amount"])
	}

	for _, path := range []string{
		"/reports/sales-rank",
		"/reports/purchase-rank",
		"/reports/customer-rank",
		"/reports/supplier-rank",
		"/reports/profit",
		"/reports/ar-ap?type=receivable",
		"/reports/purchase-detail?keyword=REP-P1",
		"/reports/sales-detail?keyword=REP-P1",
		"/reports/cash-flow",
	} {
		c.do("GET", path, nil, 0)
	}

	for _, path := range []string{
		fmt.Sprintf("/stocks/ledger/export?warehouse_id=%.0f", wid),
		"/reports/inout/export?from=" + day + "&to=" + day + "&keyword=REP-P1",
		"/reports/ar-ap/export?type=receivable",
	} {
		resp := c.rawGet(path)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("导出 %s 期望 200，实际 %d", path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); len(ct) < 8 || ct[:8] != "text/csv" {
			t.Fatalf("导出 %s Content-Type 期望 text/csv，实际 %s", path, ct)
		}
		if !bytes.HasPrefix(body, []byte{0xEF, 0xBB, 0xBF}) {
			t.Fatalf("导出 %s 应带 UTF-8 BOM", path)
		}
	}

	backup := decode[map[string]any](t, c.do("POST", "/system/backup", map[string]any{}, 0))
	if backup["name"] == nil || backup["name"].(string) == "" {
		t.Fatalf("备份应返回文件名: %v", backup)
	}
	backups := decode[[]map[string]any](t, c.do("GET", "/system/backups", nil, 0))
	if len(backups) == 0 {
		t.Fatalf("备份列表不应为空")
	}
	c.do("POST", "/system/restore", map[string]any{"name": "not-exists.db"}, http.StatusNotFound)
	c.do("POST", "/system/restore", map[string]any{"name": "../bad.txt"}, http.StatusBadRequest)

	info := decode[map[string]any](t, c.do("GET", "/system/info", nil, 0))
	if info["db_path"] == nil || info["version"] == nil {
		t.Fatalf("系统信息不完整: %v", info)
	}
}
