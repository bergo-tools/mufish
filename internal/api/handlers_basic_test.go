package api

import (
	"strings"
	"testing"
)

func TestHTTPProductImportUsesCategoryColumn(t *testing.T) {
	c, _ := newTestServer(t)
	c.login("admin", "admin123")

	cat := decode[map[string]any](t, c.do("POST", "/categories", map[string]any{
		"name": "办公用品",
	}, 0))
	catID := int64(cat["id"].(float64))

	csv := "商品编码,商品名称,规格,单位,分类,条码,品牌,参考进价,参考售价,安全库存\n" +
		"IMP-1,导入商品,标准,个,办公用品,,,10,20,5\n"
	res := decode[map[string]any](t, c.do("POST", "/products/import", map[string]any{"text": csv}, 0))
	if res["success"].(float64) != 1 || res["failed"].(float64) != 0 {
		t.Fatalf("导入应成功 1 条，实际 %v", res)
	}

	page := decode[map[string]any](t, c.do("GET", "/products?keyword=IMP-1&size=10", nil, 0))
	list := page["list"].([]any)
	if len(list) != 1 {
		t.Fatalf("应查询到 1 个导入商品，实际 %d", len(list))
	}
	row := list[0].(map[string]any)
	if int64(row["category_id"].(float64)) != catID {
		t.Fatalf("导入商品分类 ID 期望 %d，实际 %v", catID, row["category_id"])
	}
	if row["category_name"] != "办公用品" {
		t.Fatalf("导入商品分类名称期望 办公用品，实际 %v", row["category_name"])
	}

	// 不存在的分类应导致该行失败，而不是静默丢弃。
	bad := "商品编码,商品名称,规格,单位,分类,条码,品牌,参考进价,参考售价,安全库存\n" +
		"IMP-2,导入商品2,标准,个,不存在的分类,,,10,20,5\n"
	res = decode[map[string]any](t, c.do("POST", "/products/import", map[string]any{"text": bad}, 0))
	if res["success"].(float64) != 0 || res["failed"].(float64) != 1 {
		t.Fatalf("不存在的分类应导致 1 行失败，实际 %v", res)
	}
	msgs := res["messages"].([]any)
	if len(msgs) == 0 || !strings.Contains(msgs[0].(string), "分类") {
		t.Fatalf("失败信息应提示分类不存在，实际 %v", msgs)
	}
}
