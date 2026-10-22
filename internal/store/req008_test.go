package store

import (
	"context"
	"errors"
	"testing"

	"mufish/internal/model"
)

// 商品详情必须返回各仓库结存合计，前端详情页用它展示“库存合计”。
func TestGetProductIncludesStockQty(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	buy(t, st, supplier, wid, pid, 10, 10)
	saleID, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(3), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, saleID, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	p, err := st.GetProduct(ctx, pid)
	if err != nil {
		t.Fatalf("读取商品失败: %v", err)
	}
	if p.StockQty != model.QtyFrom(7) {
		t.Fatalf("商品详情库存合计期望 7，实际 %s（详情页会显示为 0）", p.StockQty)
	}
}

// 盘点差异金额必须四舍五入到分，不能截断小数分。
// 0.3333 元 × 1.5 = 0.49995 元，应按 0.50 元入账。
func TestStockTakeDiffAmountRoundsToCent(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}

	pid, err := st.SaveProduct(ctx, 0, ProductInput{
		Code: "REQ8-PRICE", Name: "小数单价盘点商品", Unit: "个",
		PurchasePrice: model.PriceFrom(0.3333), SalePrice: model.PriceFrom(1), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	wh, err := st.DefaultWarehouse(ctx)
	if err != nil {
		t.Fatalf("读取默认仓库失败: %v", err)
	}

	takeID, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wh.ID, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(1.5)}},
	}, user)
	if err != nil {
		t.Fatalf("创建盘点单失败: %v", err)
	}
	take, err := st.GetStockTake(ctx, takeID)
	if err != nil {
		t.Fatalf("读取盘点单失败: %v", err)
	}
	if len(take.Items) != 1 {
		t.Fatalf("盘点明细期望 1 行，实际 %d", len(take.Items))
	}
	if take.Items[0].DiffAmount != model.YuanFrom(0.50) {
		t.Fatalf("盘点差异金额期望 0.50 元，实际 %s（截断会得到 0.49 元）", take.Items[0].DiffAmount)
	}

	if err := st.FinishStockTake(ctx, takeID, user); err != nil {
		t.Fatalf("完成盘点失败: %v", err)
	}
	qty, amount, _ := stockState(t, st, wh.ID, pid)
	if qty != 1.5 || amount != int64(model.YuanFrom(0.50)) {
		t.Fatalf("盘点入账后期望数量 1.5 / 金额 0.50，实际 %.4f / %s", qty, model.Money(amount))
	}
}

// 调拨确认后明细金额必须同步为实际结转成本，否则详情金额对不上。
func TestTransferConfirmUpdatesItemAmount(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)
	buy(t, st, supplier, wid, pid, 10, 10)

	w2, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "REQ8-W2", Name: "调拨金额测试仓", Status: 1})
	if err != nil {
		t.Fatalf("创建仓库失败: %v", err)
	}
	tr, err := st.SaveTransfer(ctx, 0, TransferInput{
		FromWarehouseID: wid, ToWarehouseID: w2, TransferDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(3), Price: model.PriceFrom(999)}},
	}, user)
	if err != nil {
		t.Fatalf("创建调拨单失败: %v", err)
	}
	if err := st.ConfirmTransfer(ctx, tr, user); err != nil {
		t.Fatalf("确认调拨失败: %v", err)
	}
	o, err := st.GetTransfer(ctx, tr)
	if err != nil {
		t.Fatalf("读取调拨单失败: %v", err)
	}
	if o.TotalAmount != model.YuanFrom(30) {
		t.Fatalf("调拨总额期望 30，实际 %s", o.TotalAmount)
	}
	if len(o.Items) != 1 {
		t.Fatalf("调拨明细期望 1 行，实际 %d", len(o.Items))
	}
	if o.Items[0].Price != model.PriceFrom(10) || o.Items[0].Amount != model.YuanFrom(30) {
		t.Fatalf("调拨明细期望单价 10 / 金额 30，实际 %s / %s",
			o.Items[0].Price, o.Items[0].Amount)
	}
}

// 被草稿调拨/盘点引用的商品不能删除，否则确认草稿会为不存在的商品创建库存。
func TestDeleteProductBlocksDraftReferences(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	wh, err := st.DefaultWarehouse(ctx)
	if err != nil {
		t.Fatalf("读取默认仓库失败: %v", err)
	}
	w2, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "REQ8-W3", Name: "删除引用测试仓", Status: 1})
	if err != nil {
		t.Fatalf("创建仓库失败: %v", err)
	}

	// 草稿调拨引用。
	p1, err := st.SaveProduct(ctx, 0, ProductInput{Code: "REQ8-DEL-TR", Name: "调拨引用商品", Status: 1})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	tr, err := st.SaveTransfer(ctx, 0, TransferInput{
		FromWarehouseID: wh.ID, ToWarehouseID: w2, TransferDate: model.Today(),
		Items: []OrderItemInput{{ProductID: p1, Qty: model.QtyFrom(1), Price: 0}},
	}, user)
	if err != nil {
		t.Fatalf("创建草稿调拨单失败: %v", err)
	}
	if err := st.DeleteProduct(ctx, p1); !errors.Is(err, ErrInUse) {
		t.Fatalf("被草稿调拨单引用的商品应禁止删除，实际 %v", err)
	}
	if err := st.DeleteTransfer(ctx, tr); err != nil {
		t.Fatalf("删除草稿调拨单失败: %v", err)
	}
	if err := st.DeleteProduct(ctx, p1); err != nil {
		t.Fatalf("删除草稿调拨单后应可删除商品，实际 %v", err)
	}

	// 草稿盘点引用。
	p2, err := st.SaveProduct(ctx, 0, ProductInput{Code: "REQ8-DEL-TAKE", Name: "盘点引用商品", Status: 1})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	take, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wh.ID, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: p2, RealQty: model.QtyFrom(1)}},
	}, user)
	if err != nil {
		t.Fatalf("创建草稿盘点单失败: %v", err)
	}
	if err := st.DeleteProduct(ctx, p2); !errors.Is(err, ErrInUse) {
		t.Fatalf("被草稿盘点单引用的商品应禁止删除，实际 %v", err)
	}
	if err := st.DeleteStockTake(ctx, take); err != nil {
		t.Fatalf("删除草稿盘点单失败: %v", err)
	}
	if err := st.DeleteProduct(ctx, p2); err != nil {
		t.Fatalf("删除草稿盘点单后应可删除商品，实际 %v", err)
	}
}

// 停用单位仍有欠款时必须出现在应收应付对账表中，与首页口径保持一致。
func TestArApReportIncludesDisabledPartners(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	buy(t, st, supplier, wid, pid, 10, 10)
	saleID, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(5), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, saleID, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	// 停用客户与供应商，但保留欠款。
	c, err := st.GetPartner(ctx, customer)
	if err != nil {
		t.Fatalf("读取客户失败: %v", err)
	}
	if _, err := st.SavePartner(ctx, customer, PartnerInput{
		Code: c.Code, Name: c.Name, Type: c.Type, Status: 0,
	}); err != nil {
		t.Fatalf("停用客户失败: %v", err)
	}
	s, err := st.GetPartner(ctx, supplier)
	if err != nil {
		t.Fatalf("读取供应商失败: %v", err)
	}
	if _, err := st.SavePartner(ctx, supplier, PartnerInput{
		Code: s.Code, Name: s.Name, Type: s.Type, Status: 0,
	}); err != nil {
		t.Fatalf("停用供应商失败: %v", err)
	}

	ar, err := st.ArApReport(ctx, ArApQuery{Type: "receivable"})
	if err != nil {
		t.Fatalf("查询应收报表失败: %v", err)
	}
	if row, ok := findArAp(ar, c.Name); !ok || row.Balance != model.YuanFrom(100) {
		t.Fatalf("停用客户的应收余额应保留，实际 %+v / %v", row, ok)
	}
	ap, err := st.ArApReport(ctx, ArApQuery{Type: "payable"})
	if err != nil {
		t.Fatalf("查询应付报表失败: %v", err)
	}
	if row, ok := findArAp(ap, s.Name); !ok || row.Balance != model.YuanFrom(100) {
		t.Fatalf("停用供应商的应付余额应保留，实际 %+v / %v", row, ok)
	}
}
