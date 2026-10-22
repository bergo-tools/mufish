package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

// 内置财务角色只有 product:view / stock:view / report:view，没有 category:view。
// 分类下拉是这些页面的基础数据，不能因为缺少独立分类权限而 403。
func TestP1CategoryFlatPermissionMatrix(t *testing.T) {
	c, _ := newTestServer(t)

	cases := []struct {
		perm  string
		allow bool
	}{
		{"category:view", true},
		{"product:view", true},
		{"stock:view", true},
		{"report:view", true},
		{"dashboard:view", false},
	}

	for i, tc := range cases {
		c.login("admin", "admin123")
		code := fmt.Sprintf("req10_cat_%d", i)
		role := decode[map[string]any](t, c.do("POST", "/roles", map[string]any{
			"code": code, "name": code, "perms": []string{tc.perm},
		}, 0))
		username := fmt.Sprintf("req10_cat_user_%d", i)
		c.do("POST", "/users", map[string]any{
			"username": username, "password": "abc123456", "name": username,
			"role_id": role["id"], "status": 1,
		}, 0)

		c.login(username, "abc123456")
		if tc.allow {
			list := decode[[]map[string]any](t, c.do("GET", "/categories/flat", nil, 0))
			if len(list) == 0 {
				t.Fatalf("权限 %s 应能读取分类下拉，实际为空", tc.perm)
			}
		} else {
			c.do("GET", "/categories/flat", nil, http.StatusForbidden)
		}
	}
}

// 商品分类必须是真实存在的记录，禁止写入孤儿 category_id。
func TestP1ProductRejectsMissingCategory(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	c.do("POST", "/products", map[string]any{
		"code": "REQ10-BAD-CAT", "name": "错误分类商品", "category_id": 999999, "status": 1,
	}, http.StatusBadRequest)
	list := decode[map[string]any](t, c.do("GET", "/products?keyword=REQ10-BAD-CAT", nil, 0))
	if list["total"].(float64) != 0 {
		t.Fatalf("分类不存在时不应写入商品，实际总数 %v", list["total"])
	}

	pid := decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "REQ10-OK", "name": "正常商品", "status": 1,
	}, 0))["id"].(float64)
	c.do("PUT", fmt.Sprintf("/products/%d", int64(pid)), map[string]any{
		"code": "REQ10-OK", "name": "正常商品", "category_id": 999999, "status": 1,
	}, http.StatusBadRequest)
	detail := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/products/%d", int64(pid)), nil, 0))
	product := detail["product"].(map[string]any)
	if product["category_id"].(float64) != 0 {
		t.Fatalf("更新失败后分类不应改变，实际 %v", product["category_id"])
	}
}

// 收付款单的往来单位类型必须与单据方向一致，保存和确认都要校验。
func TestP1PaymentRejectsWrongPartnerRole(t *testing.T) {
	c, st := newTestServer(t)
	c.login("admin", "admin123")

	customerID := int64(decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "REQ10-C", "name": "REQ10 客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64))
	supplierID := int64(decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "REQ10-S", "name": "REQ10 供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64))

	c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": supplierID, "account_id": 1,
		"pay_date": "2026-01-05", "amount": 10, "method": "bank",
	}, http.StatusBadRequest)
	c.do("POST", "/payments", map[string]any{
		"type": "payment", "partner_id": customerID, "account_id": 1,
		"pay_date": "2026-01-05", "amount": 10, "method": "bank",
	}, http.StatusBadRequest)

	// 历史草稿即使绕过保存校验，确认时也必须再次校验，不能改动单据或账户余额。
	before, err := st.GetAccount(context.Background(), 1)
	if err != nil {
		t.Fatalf("读取账户失败: %v", err)
	}
	res, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO payments(payment_no, type, partner_id, partner_name, account_id, account_name,
		   pay_date, amount, method, order_type, order_id, order_no, status, remark,
		   created_by, created_by_name, created_at, updated_at)
		 VALUES('REQ10-BAD-PAY','receipt',?,?,1,'现金','2026-01-05',1000,'bank','',0,'',0,'草稿',1,'测试','2026-01-05 00:00:00','2026-01-05 00:00:00')`,
		supplierID, "REQ10 供应商")
	if err != nil {
		t.Fatalf("插入历史错误草稿失败: %v", err)
	}
	pid, _ := res.LastInsertId()

	c.do("POST", fmt.Sprintf("/payments/%d/confirm", pid), map[string]any{}, http.StatusBadRequest)
	payment := decode[map[string]any](t, c.do("GET", fmt.Sprintf("/payments/%d", pid), nil, 0))
	if payment["status"].(float64) != 0 {
		t.Fatalf("确认失败后单据应保持草稿，实际状态 %v", payment["status"])
	}
	after, err := st.GetAccount(context.Background(), 1)
	if err != nil {
		t.Fatalf("读取账户失败: %v", err)
	}
	if after.Balance != before.Balance {
		t.Fatalf("确认失败后账户余额不应变化，前 %s 后 %s", before.Balance, after.Balance)
	}
}

// 修改往来单位类型不能把已有业务的欠款从应收/应付报表中隐藏。
func TestP1PartnerTypeChangeKeepsExistingBalances(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	pid := int64(decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "REQ10-P", "name": "REQ10 商品", "purchase_price": 10, "sale_price": 20, "status": 1,
	}, 0))["id"].(float64))
	wid := int64(decode[[]map[string]any](t, c.do("GET", "/warehouses/options", nil, 0))[0]["id"].(float64))
	cid := int64(decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "REQ10-TC", "name": "REQ10 类型客户", "type": "customer", "status": 1,
	}, 0))["id"].(float64))
	sid := int64(decode[map[string]any](t, c.do("POST", "/partners", map[string]any{
		"code": "REQ10-TS", "name": "REQ10 类型供应商", "type": "supplier", "status": 1,
	}, 0))["id"].(float64))

	po := decode[map[string]any](t, c.do("POST", "/purchases", map[string]any{
		"supplier_id": sid, "warehouse_id": wid, "order_date": "2026-01-05",
		"items": []map[string]any{{"product_id": pid, "qty": 10, "price": 10}},
	}, 0))
	c.do("POST", fmt.Sprintf("/purchases/%d/confirm", int64(po["id"].(float64))), map[string]any{}, 0)
	so := decode[map[string]any](t, c.do("POST", "/sales", map[string]any{
		"customer_id": cid, "warehouse_id": wid, "order_date": "2026-01-06",
		"items": []map[string]any{{"product_id": pid, "qty": 1, "price": 20}},
	}, 0))
	c.do("POST", fmt.Sprintf("/sales/%d/confirm", int64(so["id"].(float64))), map[string]any{}, 0)

	c.do("PUT", fmt.Sprintf("/partners/%d", cid), map[string]any{
		"code": "REQ10-TC", "name": "REQ10 类型客户", "type": "supplier", "status": 1,
	}, http.StatusBadRequest)
	c.do("PUT", fmt.Sprintf("/partners/%d", sid), map[string]any{
		"code": "REQ10-TS", "name": "REQ10 类型供应商", "type": "customer", "status": 1,
	}, http.StatusBadRequest)

	// 改为 both 不影响已有应收/应付，应该允许。
	c.do("PUT", fmt.Sprintf("/partners/%d", cid), map[string]any{
		"code": "REQ10-TC", "name": "REQ10 类型客户", "type": "both", "status": 1,
	}, 0)
	c.do("PUT", fmt.Sprintf("/partners/%d", sid), map[string]any{
		"code": "REQ10-TS", "name": "REQ10 类型供应商", "type": "both", "status": 1,
	}, 0)

	ar := decode[[]map[string]any](t, c.do("GET", "/reports/ar-ap?type=receivable", nil, 0))
	found := false
	for _, row := range ar {
		if row["partner_name"] == "REQ10 类型客户" {
			found = true
			if row["balance"].(float64) != 20 {
				t.Fatalf("客户应收应保持 20，实际 %v", row["balance"])
			}
		}
	}
	if !found {
		t.Fatal("修改类型后应收报表不应丢失该客户")
	}
}

// 有盘点单引用的仓库不能删除，否则盘点完成时会把库存写到不存在的仓库。
func TestP1WarehouseWithStockTakeCannotBeDeleted(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	wid := int64(decode[map[string]any](t, c.do("POST", "/warehouses", map[string]any{
		"code": "REQ10-WH", "name": "REQ10 盘点仓", "status": 1,
	}, 0))["id"].(float64))
	pid := int64(decode[map[string]any](t, c.do("POST", "/products", map[string]any{
		"code": "REQ10-WHP", "name": "REQ10 盘点商品", "status": 1,
	}, 0))["id"].(float64))
	takeID := int64(decode[map[string]any](t, c.do("POST", "/stock-takes", map[string]any{
		"warehouse_id": wid, "take_date": "2026-01-07",
		"items": []map[string]any{{"product_id": pid, "real_qty": 0}},
	}, 0))["id"].(float64))

	c.do("DELETE", fmt.Sprintf("/warehouses/%d", wid), nil, http.StatusConflict)
	c.do("DELETE", fmt.Sprintf("/stock-takes/%d", takeID), nil, 0)
	c.do("DELETE", fmt.Sprintf("/warehouses/%d", wid), nil, 0)
}
