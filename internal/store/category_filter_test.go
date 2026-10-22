package store

import (
	"context"
	"testing"

	"mufish/internal/model"
)

// 按分类过滤时必须包含任意层级的后代分类。
func TestCategoryFilterIncludesAllDescendants(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}

	a, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "一级分类"})
	if err != nil {
		t.Fatalf("创建一级分类失败: %v", err)
	}
	b, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "二级分类", ParentID: a})
	if err != nil {
		t.Fatalf("创建二级分类失败: %v", err)
	}
	c, err := st.SaveCategory(ctx, 0, CategoryInput{Name: "三级分类", ParentID: b})
	if err != nil {
		t.Fatalf("创建三级分类失败: %v", err)
	}
	pid, err := st.SaveProduct(ctx, 0, ProductInput{
		Code: "CAT-P1", Name: "三级分类商品", Unit: "个", CategoryID: c,
		PurchasePrice: model.PriceFrom(10), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建三级分类商品失败: %v", err)
	}
	supplier, err := st.SavePartner(ctx, 0, PartnerInput{Code: "CAT-S1", Name: "分类供应商", Type: "supplier", Status: 1})
	if err != nil {
		t.Fatalf("创建供应商失败: %v", err)
	}
	wh, err := st.DefaultWarehouse(ctx)
	if err != nil {
		t.Fatalf("读取默认仓库失败: %v", err)
	}
	purchase, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wh.ID, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, purchase, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}

	products, err := st.ListProducts(ctx, ProductQuery{CategoryID: a, Status: -1, Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("按一级分类查询商品失败: %v", err)
	}
	found := false
	for _, p := range products.List {
		if p.ID == pid {
			found = true
		}
	}
	if !found {
		t.Fatalf("按一级分类查询应包含三级分类商品，实际 %+v", products.List)
	}

	stocks, err := st.ListStocks(ctx, StockQuery{CategoryID: a, Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("按一级分类查询库存失败: %v", err)
	}
	found = false
	for _, s := range stocks.List {
		if s.ProductID == pid {
			found = true
		}
	}
	if !found {
		t.Fatalf("按一级分类查询库存应包含三级分类商品，实际 %+v", stocks.List)
	}

	summary, err := st.StockSummary(ctx, wh.ID, a)
	if err != nil {
		t.Fatalf("按一级分类查询库存汇总失败: %v", err)
	}
	if summary["kinds"].(int) != 1 || summary["qty"].(model.Qty) != 10 {
		t.Fatalf("按一级分类汇总应只包含三级分类商品，实际 %+v", summary)
	}

	inout, err := st.InoutSummary(ctx, InoutQuery{CategoryID: a})
	if err != nil {
		t.Fatalf("按一级分类查询进销存汇总失败: %v", err)
	}
	found = false
	for _, r := range inout {
		if r.Code == "CAT-P1" {
			found = true
			if r.InQty != 10 || r.EndQty != 10 {
				t.Fatalf("三级分类商品进销存数据不正确: %+v", r)
			}
		}
	}
	if !found {
		t.Fatalf("按一级分类查询进销存应包含三级分类商品，实际 %+v", inout)
	}
}
