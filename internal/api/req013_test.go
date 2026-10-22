package api

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"mufish/internal/model"
	"mufish/internal/store"
)

// 停用的往来单位 / 资金账户通过 HTTP 新建单据时也必须返回 400，不能只在前端隐藏下拉。
func TestHTTPDisabledMasterDataRejectedForNewDocuments(t *testing.T) {
	c, st := newTestServer(t)
	c.login("admin", "admin123")

	pid := decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "API013-P1", "name": "停用校验商品", "unit": "个",
		"purchase_price": 10, "sale_price": 20, "safety_stock": 0, "status": 1,
	}, 0))["id"].(float64)

	sid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "API013-S1", "name": "停用校验供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64)
	c.do("PUT", "/partners/"+strconv.FormatInt(int64(sid), 10), map[string]any{
		"code": "API013-S1", "name": "停用校验供应商", "type": "supplier", "status": 0,
	}, 0)

	whs := decode[[]map[string]any](t, c.do("GET", "/warehouses/options", nil, 0))
	if len(whs) == 0 {
		t.Fatalf("应存在默认仓库")
	}
	wid := int64(whs[0]["id"].(float64))

	c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": pid, "qty": 1, "price": 10}},
	}, http.StatusBadRequest)
	page, err := st.ListPurchases(context.Background(), store.PurchaseQuery{Status: -1, Page: 1, Size: 10})
	if err != nil {
		t.Fatalf("查询采购单失败: %v", err)
	}
	if page.Total != 0 {
		t.Fatalf("停用供应商的采购单不应写入，实际 %d 条", page.Total)
	}

	cid := decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "API013-C1", "name": "停用校验客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64)
	aid := decode[map[string]any](t, c.do("POST", "/accounts", map[string]any{
		"code": "API013-A1", "name": "停用校验账户", "type": "cash", "status": 0,
	}, 0))["id"].(float64)

	c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": cid, "account_id": aid,
		"pay_date": model.Today(), "amount": 10, "method": "bank",
	}, http.StatusBadRequest)
	payPage, err := st.ListPayments(context.Background(), store.PaymentQuery{Status: -1, Page: 1, Size: 10})
	if err != nil {
		t.Fatalf("查询收付款单失败: %v", err)
	}
	if payPage.Total != 0 {
		t.Fatalf("停用账户的收款单不应写入，实际 %d 条", payPage.Total)
	}
}
