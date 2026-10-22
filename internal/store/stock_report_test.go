package store

import (
	"context"
	"testing"

	"mufish/internal/model"
)

func findArAp(rows []model.ArApRow, name string) (model.ArApRow, bool) {
	for _, r := range rows {
		if r.PartnerName == name {
			return r, true
		}
	}
	return model.ArApRow{}, false
}

func TestStockQueriesAndReports(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, _ := bootstrap(t, st)

	// 使用独立仓库，避免演示数据的期初库存干扰统计。
	wid, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "REP-WH", Name: "报表测试仓", Status: 1})
	if err != nil {
		t.Fatalf("创建测试仓库失败: %v", err)
	}

	p, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	s, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(3), Price: model.PriceFrom(20)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, s, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	// 库存查询。
	stocks, err := st.ListStocks(ctx, StockQuery{WarehouseID: wid, Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("查询库存失败: %v", err)
	}
	if stocks.Total != 1 {
		t.Fatalf("测试仓库应有 1 条库存，实际 %d", stocks.Total)
	}
	if stocks.List[0].Qty != 7 || stocks.List[0].CostPrice != model.PriceFrom(10) || stocks.List[0].Amount != model.YuanFrom(70) {
		t.Fatalf("库存余额不正确: %+v", stocks.List[0])
	}
	if stocks.List[0].WarehouseName != "报表测试仓" || stocks.List[0].CategoryName == "" && stocks.List[0].ProductName == "" {
		t.Fatalf("库存关联字段不正确: %+v", stocks.List[0])
	}

	byWarehouse, err := st.ProductStockByWarehouse(ctx, pid)
	if err != nil {
		t.Fatalf("查询商品分仓库存失败: %v", err)
	}
	found := false
	for _, x := range byWarehouse {
		if x.WarehouseID == wid {
			found = true
			if x.Qty != 7 {
				t.Fatalf("分仓库存期望 7，实际 %s", x.Qty)
			}
		}
	}
	if !found {
		t.Fatalf("分仓库存应包含测试仓库")
	}

	summary, err := st.StockSummary(ctx, wid, 0)
	if err != nil {
		t.Fatalf("查询库存汇总失败: %v", err)
	}
	if summary["kinds"].(int) != 1 || summary["qty"].(model.Qty) != 7 || summary["amount"].(model.Money) != model.YuanFrom(70) {
		t.Fatalf("库存汇总不正确: %+v", summary)
	}

	// 库存流水与分页。
	moves, err := st.ListStockMoves(ctx, StockLedgerQuery{WarehouseID: wid, Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("查询库存流水失败: %v", err)
	}
	if moves.Total != 2 || len(moves.List) != 2 {
		t.Fatalf("库存流水应有 2 条，实际 %d / %d", moves.Total, len(moves.List))
	}
	if moves.List[0].Direction != -1 {
		t.Fatalf("库存流水应按 ID 倒序，最新一条应为出库")
	}
	purchases, err := st.ListStockMoves(ctx, StockLedgerQuery{WarehouseID: wid, BizType: "purchase", Page: 1, Size: 20})
	if err != nil || purchases.Total != 1 {
		t.Fatalf("采购流水过滤期望 1 条，实际 %d / %v", purchases.Total, err)
	}
	today, err := st.ListStockMoves(ctx, StockLedgerQuery{WarehouseID: wid, From: model.Today(), To: model.Today(), Page: 1, Size: 20})
	if err != nil || today.Total != 2 {
		t.Fatalf("按日期过滤期望 2 条，实际 %d / %v", today.Total, err)
	}
	paged, err := st.ListStockMoves(ctx, StockLedgerQuery{WarehouseID: wid, Page: 1, Size: 1})
	if err != nil || len(paged.List) != 1 || paged.Total != 2 {
		t.Fatalf("流水分页不正确: %d / %d / %v", len(paged.List), paged.Total, err)
	}

	// 安全库存 10 > 结存 7，应进入预警。
	if _, err := st.SaveProduct(ctx, pid, ProductInput{
		Name: "测试商品A", Spec: "标准", Unit: "个",
		PurchasePrice: model.PriceFrom(10), SalePrice: model.PriceFrom(20),
		SafetyStock: model.QtyFrom(10), Status: 1,
	}); err != nil {
		t.Fatalf("更新商品安全库存失败: %v", err)
	}
	low, err := st.LowStockList(ctx, wid, 100)
	if err != nil {
		t.Fatalf("查询库存预警失败: %v", err)
	}
	var lowRow *model.Stock
	for i := range low {
		if low[i].ProductID == pid {
			lowRow = &low[i]
		}
	}
	if lowRow == nil || lowRow.Qty != 7 || lowRow.SafetyStock != 10 {
		t.Fatalf("库存预警不正确: %+v", low)
	}
	lowStocks, err := st.ListStocks(ctx, StockQuery{WarehouseID: wid, OnlyLow: true, Page: 1, Size: 100})
	if err != nil {
		t.Fatalf("仅低库存过滤失败: %v", err)
	}
	if lowStocks.Total != int64(len(low)) {
		t.Fatalf("库存查询低库存口径应与预警一致，查询 %d 条，预警 %d 条", lowStocks.Total, len(low))
	}
	foundLow := false
	for _, s := range lowStocks.List {
		if s.ProductID == pid {
			foundLow = true
			if s.Qty != 7 || s.SafetyStock != 10 {
				t.Fatalf("低库存查询数据不正确: %+v", s)
			}
		}
	}
	if !foundLow {
		t.Fatalf("低库存查询应包含测试商品，实际 %+v", lowStocks.List)
	}

	// 看板。
	dash, err := st.Dashboard(ctx)
	if err != nil {
		t.Fatalf("查询看板失败: %v", err)
	}
	if dash.TodaySales != model.YuanFrom(60) || dash.TodayPurchase != model.YuanFrom(100) {
		t.Fatalf("看板今日销售/采购不正确: %+v", dash)
	}
	if dash.MonthProfit != model.YuanFrom(30) {
		t.Fatalf("看板月毛利期望 30，实际 %s", dash.MonthProfit)
	}
	if dash.Receivable != model.YuanFrom(60) || dash.Payable != model.YuanFrom(100) {
		t.Fatalf("看板应收应付不正确: 应收 %s 应付 %s", dash.Receivable, dash.Payable)
	}
	if dash.LowStockCount != 1 {
		t.Fatalf("看板低库存数量期望 1，实际 %d", dash.LowStockCount)
	}
	if dash.StockAmount < model.YuanFrom(70) || dash.StockKinds < 1 {
		t.Fatalf("看板库存数据不正确: %+v", dash)
	}

	// 走势：最后一天应是今天。
	trend, err := st.Trend(ctx, 14)
	if err != nil || len(trend) != 14 {
		t.Fatalf("走势应有 14 天，实际 %d / %v", len(trend), err)
	}
	last := trend[len(trend)-1]
	if last.Date != model.Today() || last.Sales != model.YuanFrom(60) || last.Purchase != model.YuanFrom(100) || last.Profit != model.YuanFrom(30) {
		t.Fatalf("今日走势不正确: %+v", last)
	}

	// 排行与毛利。
	salesRank, err := st.SalesRank(ctx, RankQuery{Limit: 10})
	if err != nil || len(salesRank) != 1 {
		t.Fatalf("销售排行应有 1 行，实际 %d / %v", len(salesRank), err)
	}
	if salesRank[0].Qty != 3 || salesRank[0].Amount != model.YuanFrom(60) ||
		salesRank[0].Cost != model.YuanFrom(30) || salesRank[0].Profit != model.YuanFrom(30) {
		t.Fatalf("销售排行数据不正确: %+v", salesRank[0])
	}
	customerRank, err := st.CustomerRank(ctx, RankQuery{Limit: 10})
	if err != nil || len(customerRank) != 1 {
		t.Fatalf("客户排行应有 1 行，实际 %d / %v", len(customerRank), err)
	}
	if customerRank[0].Name != "测试客户" || customerRank[0].Amount != model.YuanFrom(60) {
		t.Fatalf("客户排行数据不正确: %+v", customerRank[0])
	}
	supplierRank, err := st.SupplierRank(ctx, RankQuery{Limit: 10})
	if err != nil || len(supplierRank) != 1 {
		t.Fatalf("供应商排行应有 1 行，实际 %d / %v", len(supplierRank), err)
	}
	if supplierRank[0].Name != "测试供应商" || supplierRank[0].Amount != model.YuanFrom(100) {
		t.Fatalf("供应商排行数据不正确: %+v", supplierRank[0])
	}
	purchaseRank, err := st.PurchaseRank(ctx, RankQuery{Limit: 10})
	if err != nil || len(purchaseRank) != 1 {
		t.Fatalf("采购排行应有 1 行，实际 %d / %v", len(purchaseRank), err)
	}
	if purchaseRank[0].Qty != 10 || purchaseRank[0].Amount != model.YuanFrom(100) {
		t.Fatalf("采购排行数据不正确: %+v", purchaseRank[0])
	}
	profit, err := st.ProfitByProduct(ctx, RankQuery{Limit: 10})
	if err != nil || len(profit) != 1 || profit[0].Profit != model.YuanFrom(30) {
		t.Fatalf("商品毛利数据不正确: %+v / %v", profit, err)
	}

	// 应收应付。
	ar, err := st.ArApReport(ctx, ArApQuery{Type: "receivable"})
	if err != nil {
		t.Fatalf("查询应收报表失败: %v", err)
	}
	arRow, ok := findArAp(ar, "测试客户")
	if !ok || arRow.Balance != model.YuanFrom(60) {
		t.Fatalf("应收报表不正确: %+v / %v", arRow, ok)
	}
	ap, err := st.ArApReport(ctx, ArApQuery{Type: "payable"})
	if err != nil {
		t.Fatalf("查询应付报表失败: %v", err)
	}
	apRow, ok := findArAp(ap, "测试供应商")
	if !ok || apRow.Balance != model.YuanFrom(100) {
		t.Fatalf("应付报表不正确: %+v / %v", apRow, ok)
	}

	// 明细报表。
	purchaseDetail, err := st.PurchaseDetail(ctx, DocDetailQuery{Keyword: "T-P001", Limit: 10})
	if err != nil || len(purchaseDetail) != 1 || purchaseDetail[0].Qty != 10 {
		t.Fatalf("采购明细不正确: %+v / %v", purchaseDetail, err)
	}
	salesDetail, err := st.SalesDetail(ctx, DocDetailQuery{Keyword: "T-P001", Limit: 10})
	if err != nil || len(salesDetail) != 1 || salesDetail[0].Qty != 3 {
		t.Fatalf("销售明细不正确: %+v / %v", salesDetail, err)
	}

	// 收付款与资金流水。
	accounts, err := st.ListAccounts(ctx, true)
	if err != nil || len(accounts) == 0 {
		t.Fatalf("应存在资金账户: %v", err)
	}
	receipt, err := st.SavePayment(ctx, 0, PaymentInput{
		Type: model.PayTypeReceipt, PartnerID: customer, AccountID: accounts[0].ID,
		PayDate: model.Today(), Amount: model.YuanFrom(20), Method: "bank",
		OrderType: "sales", OrderID: s,
	}, user)
	if err != nil {
		t.Fatalf("创建收款单失败: %v", err)
	}
	if err := st.ConfirmPayment(ctx, receipt, user); err != nil {
		t.Fatalf("确认收款单失败: %v", err)
	}
	cash, err := st.CashFlow(ctx, model.Today(), model.Today(), accounts[0].ID, 10)
	if err != nil || len(cash) != 1 || cash[0].Amount != model.YuanFrom(20) {
		t.Fatalf("资金流水不正确: %+v / %v", cash, err)
	}
	ar, _ = st.ArApReport(ctx, ArApQuery{Type: "receivable"})
	arRow, _ = findArAp(ar, "测试客户")
	if arRow.Balance != model.YuanFrom(40) {
		t.Fatalf("收款后应收期望 40，实际 %s", arRow.Balance)
	}
	unpaid, err := st.UnpaidOrders(ctx, "sales", customer)
	if err != nil || len(unpaid) != 1 || unpaid[0]["balance"] != model.YuanFrom(40) {
		t.Fatalf("未结销售单不正确: %+v / %v", unpaid, err)
	}
	if err := st.VoidPayment(ctx, receipt, user); err != nil {
		t.Fatalf("作废收款单失败: %v", err)
	}
	ar, _ = st.ArApReport(ctx, ArApQuery{Type: "receivable"})
	arRow, _ = findArAp(ar, "测试客户")
	if arRow.Balance != model.YuanFrom(60) {
		t.Fatalf("作废收款后应收期望 60，实际 %s", arRow.Balance)
	}
}
