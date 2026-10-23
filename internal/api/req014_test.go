package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mufish/internal/model"
	"mufish/internal/store"
)

// 同一秒内连续备份必须生成不同文件名；否则 VACUUM INTO 会因目标文件
// 已存在而失败，用户第二次点击备份会看到 500。
func TestNextBackupNameAvoidsSameSecondCollision(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 10, 23, 10, 30, 0, 0, time.Local)

	name1, err := nextBackupName(dir, now)
	if err != nil {
		t.Fatalf("生成第一个备份文件名失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name1), []byte("backup"), 0o644); err != nil {
		t.Fatalf("写入备份文件失败: %v", err)
	}

	name2, err := nextBackupName(dir, now)
	if err != nil {
		t.Fatalf("生成第二个备份文件名失败: %v", err)
	}
	if name1 == name2 {
		t.Fatalf("同一秒内两次备份不应返回相同文件名: %s", name1)
	}
	if _, err := os.Stat(filepath.Join(dir, name2)); !os.IsNotExist(err) {
		t.Fatalf("第二个备份文件名不应指向已存在文件: %s", name2)
	}
}

func TestHTTPRejectsNonFiniteNumbers(t *testing.T) {
	c, st := newTestServer(t)
	ctx := context.Background()
	c.login("admin", "admin123")

	productID, err := st.SaveProduct(ctx, 0, store.ProductInput{
		Code: "REQ014-NUM", Name: "数值校验商品", Unit: "个",
		PurchasePrice: model.PriceFrom(10), SalePrice: model.PriceFrom(20), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	customerID, err := st.SavePartner(ctx, 0, store.PartnerInput{
		Code: "REQ014-CUS", Name: "数值校验客户", Type: "customer", Status: 1,
	})
	if err != nil {
		t.Fatalf("创建客户失败: %v", err)
	}
	wh, err := st.DefaultWarehouse(ctx)
	if err != nil {
		t.Fatalf("读取默认仓库失败: %v", err)
	}
	accounts, err := st.ListAccounts(ctx, false)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("读取资金账户失败: %v", err)
	}

	c.do("POST", "/sales", map[string]any{
		"customer_id": customerID, "warehouse_id": wh.ID, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": productID, "qty": "NaN", "price": 20}},
	}, http.StatusBadRequest)
	c.do("POST", "/sales", map[string]any{
		"customer_id": customerID, "warehouse_id": wh.ID, "order_date": model.Today(),
		"items": []map[string]any{{"product_id": productID, "qty": 1, "price": "Inf"}},
	}, http.StatusBadRequest)
	c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": customerID, "account_id": accounts[0].ID,
		"pay_date": model.Today(), "method": "bank", "amount": "-Inf",
	}, http.StatusBadRequest)

	var sales, payments int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sales_orders`).Scan(&sales); err != nil {
		t.Fatalf("统计销售单失败: %v", err)
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM payments`).Scan(&payments); err != nil {
		t.Fatalf("统计收付款单失败: %v", err)
	}
	if sales != 0 || payments != 0 {
		t.Fatalf("非法数值不应写入单据，销售 %d 收付款 %d", sales, payments)
	}
}

func TestHTTPRejectsInvalidDocumentDate(t *testing.T) {
	c, st := newTestServer(t)
	ctx := context.Background()
	c.login("admin", "admin123")

	productID, err := st.SaveProduct(ctx, 0, store.ProductInput{
		Code: "REQ014-DATE-P", Name: "日期校验商品", Unit: "个",
		PurchasePrice: model.PriceFrom(10), SalePrice: model.PriceFrom(20), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	customerID, err := st.SavePartner(ctx, 0, store.PartnerInput{
		Code: "REQ014-DATE-C", Name: "日期校验客户", Type: "customer", Status: 1,
	})
	if err != nil {
		t.Fatalf("创建客户失败: %v", err)
	}
	wh, err := st.DefaultWarehouse(ctx)
	if err != nil {
		t.Fatalf("读取默认仓库失败: %v", err)
	}
	accounts, err := st.ListAccounts(ctx, false)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("读取资金账户失败: %v", err)
	}

	c.do("POST", "/sales", map[string]any{
		"customer_id": customerID, "warehouse_id": wh.ID, "order_date": "2026-13-99",
		"items": []map[string]any{{"product_id": productID, "qty": 1, "price": 20}},
	}, http.StatusBadRequest)
	c.do("POST", "/payments", map[string]any{
		"type": "receipt", "partner_id": customerID, "account_id": accounts[0].ID,
		"pay_date": "2026-02-30", "method": "bank", "amount": 10,
	}, http.StatusBadRequest)

	var sales, payments int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM sales_orders`).Scan(&sales); err != nil {
		t.Fatalf("统计销售单失败: %v", err)
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM payments`).Scan(&payments); err != nil {
		t.Fatalf("统计收付款单失败: %v", err)
	}
	if sales != 0 || payments != 0 {
		t.Fatalf("非法日期不应写入单据，销售 %d 收付款 %d", sales, payments)
	}
}

func TestHTTPInvalidUserRoleRejected(t *testing.T) {
	c, st := newTestServer(t)
	ctx := context.Background()
	c.login("admin", "admin123")

	c.do("POST", "/users", map[string]any{
		"username": "req014_bad_role_api", "password": "abc123456", "role_id": 999999, "status": 1,
	}, http.StatusBadRequest)

	var count int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE username=?`, "req014_bad_role_api").Scan(&count); err != nil {
		t.Fatalf("统计用户失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("非法角色不应创建用户，实际 %d 条", count)
	}
}

func TestHTTPConsecutiveBackupsDoNotCollide(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	first := decode[map[string]any](t, c.do("POST", "/system/backup", map[string]any{}, http.StatusOK))
	second := decode[map[string]any](t, c.do("POST", "/system/backup", map[string]any{}, http.StatusOK))
	if first["name"] == second["name"] {
		t.Fatalf("连续备份返回了相同文件名: %v", first["name"])
	}

	list := decode[[]map[string]any](t, c.do("GET", "/system/backups", nil, http.StatusOK))
	if len(list) < 2 {
		t.Fatalf("连续备份后应至少有 2 个文件，实际 %d", len(list))
	}
}
