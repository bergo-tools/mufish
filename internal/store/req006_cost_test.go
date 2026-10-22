package store

import (
	"context"
	"errors"
	"testing"

	"mufish/internal/model"
)

// P1-26：默认口径下，采购运费计入成本、商业折扣冲减成本。
func TestP1PurchaseCostIncludesFeeAndDiscount(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		DiscountAmount: model.YuanFrom(100), OtherFee: model.YuanFrom(50),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(100)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	got, _ := st.GetPurchase(ctx, po)
	if got.SettleAmount != model.YuanFrom(950) {
		t.Fatalf("结算金额期望 950 元，实际 %s", got.SettleAmount)
	}
	if len(got.Items) != 1 || got.Items[0].Price != model.PriceFrom(100) {
		t.Fatalf("明细单价应保留供应商原价 100 元，实际 %+v", got.Items)
	}
	stocks, err := st.ProductStockByWarehouse(ctx, pid)
	if err != nil {
		t.Fatalf("查询库存失败: %v", err)
	}
	var found bool
	for _, s := range stocks {
		if s.WarehouseID != wid {
			continue
		}
		found = true
		if s.Qty != model.QtyFrom(10) {
			t.Fatalf("库存数量期望 10，实际 %s", s.Qty)
		}
		if s.Amount != model.YuanFrom(950) {
			t.Fatalf("库存金额期望 950 元（1000-100+50），实际 %s", s.Amount)
		}
		if s.CostPrice != model.PriceFrom(95) {
			t.Fatalf("库存成本价期望 95 元，实际 %s", s.CostPrice)
		}
	}
	if !found {
		t.Fatal("未找到入库库存")
	}
}

// P1-26：多行采购按金额比例分摊其他费用。
func TestP1PurchaseCostAllocatesAcrossLines(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	p1, supplier, _, wid := bootstrap(t, st)
	p2, err := st.SaveProduct(ctx, 0, ProductInput{
		Code: "P1-COST-2", Name: "第二商品", Unit: "个", PurchasePrice: model.PriceFrom(300), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建第二商品失败: %v", err)
	}

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		OtherFee: model.YuanFrom(40),
		Items: []OrderItemInput{
			{ProductID: p1, Qty: model.QtyFrom(1), Price: model.PriceFrom(100)},
			{ProductID: p2, Qty: model.QtyFrom(1), Price: model.PriceFrom(300)},
		},
	}, user)
	if err != nil {
		t.Fatalf("创建多行采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}

	want := map[int64]model.Money{p1: model.YuanFrom(110), p2: model.YuanFrom(330)}
	for productID, wantAmount := range want {
		stocks, err := st.ProductStockByWarehouse(ctx, productID)
		if err != nil {
			t.Fatalf("查询商品 %d 库存失败: %v", productID, err)
		}
		var got model.Money
		for _, s := range stocks {
			if s.WarehouseID == wid {
				got = s.Amount
			}
		}
		if got != wantAmount {
			t.Fatalf("商品 %d 分摊后库存金额期望 %s，实际 %s", productID, wantAmount, got)
		}
	}
}

// P1-26：分摊到分时使用最大余数法，各行净额之和必须精确等于目标。
func TestP1PurchaseCostAllocationRoundsToCents(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	p1, supplier, _, wid := bootstrap(t, st)
	p2, err := st.SaveProduct(ctx, 0, ProductInput{
		Code: "P1-COST-2", Name: "第二商品", Unit: "个", Status: 1,
	})
	if err != nil {
		t.Fatalf("创建第二商品失败: %v", err)
	}

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		OtherFee: model.YuanFrom(0.01),
		Items: []OrderItemInput{
			{ProductID: p1, Qty: model.QtyFrom(1), Price: model.PriceFrom(0.01)},
			{ProductID: p2, Qty: model.QtyFrom(1), Price: model.PriceFrom(0.01)},
		},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	var total model.Money
	for _, productID := range []int64{p1, p2} {
		stocks, _ := st.ProductStockByWarehouse(ctx, productID)
		for _, s := range stocks {
			if s.WarehouseID == wid {
				total += s.Amount
			}
		}
	}
	if total != model.YuanFrom(0.03) {
		t.Fatalf("两行库存金额之和期望 0.03 元（0.01+0.01+0.01），实际 %s", total)
	}
}

// P1-26：可在系统参数关闭“采购费用计入库存成本”，关闭后保持原逻辑。
func TestP1PurchaseCostSettingOff(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)
	if err := st.SaveSettings(ctx, map[string]string{"purchase_cost_include": "0"}); err != nil {
		t.Fatalf("关闭采购费用计入成本失败: %v", err)
	}

	po, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		OtherFee: model.YuanFrom(50),
		Items:    []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(100)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, po, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	got, _ := st.GetPurchase(ctx, po)
	if got.SettleAmount != model.YuanFrom(1050) {
		t.Fatalf("关闭计入成本后结算金额仍应为 1050，实际 %s", got.SettleAmount)
	}
	stocks, _ := st.ProductStockByWarehouse(ctx, pid)
	for _, s := range stocks {
		if s.WarehouseID != wid {
			continue
		}
		if s.Amount != model.YuanFrom(1000) || s.CostPrice != model.PriceFrom(100) {
			t.Fatalf("关闭计入成本后库存应保持 1000 元/100 元，实际 %s/%s", s.Amount, s.CostPrice)
		}
	}
}

// P1-26：优惠金额超过明细金额时拒绝，避免负库存成本。
func TestP1PurchaseDiscountCannotExceedTotal(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)
	_, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		DiscountAmount: model.YuanFrom(11),
		Items:          []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(1), Price: model.PriceFrom(10)}},
	}, user)
	if !errors.Is(err, ErrBadRequest) {
		t.Fatalf("优惠超过明细金额应返回 ErrBadRequest，实际 %v", err)
	}
	page, _ := st.ListPurchases(ctx, PurchaseQuery{Status: -1, Page: 1, Size: 10})
	if page.Total != 0 {
		t.Fatalf("失败的采购不应写入数据，实际 %d 条", page.Total)
	}
}
