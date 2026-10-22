package store

import (
	"context"
	"testing"

	"mufish/internal/model"
)

// 库存流水的业务日期必须取单据日期，而不是确认时间。
func TestStockMoveBizDateFollowsDocumentDate(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, customer, wid := bootstrap(t, st)

	purchase, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: "2001-01-05",
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(10)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, purchase, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	sale, err := st.SaveSales(ctx, 0, SalesInput{
		CustomerID: customer, WarehouseID: wid, OrderDate: "2001-01-10",
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(4), Price: model.PriceFrom(15)}},
	}, user)
	if err != nil {
		t.Fatalf("创建销售单失败: %v", err)
	}
	if err := st.ConfirmSales(ctx, sale, user); err != nil {
		t.Fatalf("确认销售单失败: %v", err)
	}

	moves, err := st.ListStockMoves(ctx, StockLedgerQuery{WarehouseID: wid, Page: 1, Size: 10})
	if err != nil {
		t.Fatalf("查询库存流水失败: %v", err)
	}
	var sawPurchase, sawSale bool
	for _, m := range moves.List {
		switch m.BizType {
		case "purchase":
			sawPurchase = true
			if m.BizDate != "2001-01-05" {
				t.Fatalf("采购流水业务日期期望 2001-01-05，实际 %q", m.BizDate)
			}
		case "sales":
			sawSale = true
			if m.BizDate != "2001-01-10" {
				t.Fatalf("销售流水业务日期期望 2001-01-10，实际 %q", m.BizDate)
			}
		}
	}
	if !sawPurchase || !sawSale {
		t.Fatalf("未找到采购或销售流水: %+v", moves.List)
	}

	jan, err := st.InoutSummary(ctx, InoutQuery{
		From: "2001-01-01", To: "2001-01-31", Keyword: "T-P001",
	})
	if err != nil {
		t.Fatalf("查询 2001 年 1 月汇总失败: %v", err)
	}
	if len(jan) != 1 || jan[0].InQty != 10 || jan[0].OutQty != 4 || jan[0].EndQty != 6 {
		t.Fatalf("1 月汇总应包含历史单据，实际 %+v", jan)
	}

	feb, err := st.InoutSummary(ctx, InoutQuery{
		From: "2001-02-01", To: "2001-02-28", Keyword: "T-P001",
	})
	if err != nil {
		t.Fatalf("查询 2001 年 2 月汇总失败: %v", err)
	}
	if len(feb) != 1 || feb[0].InitQty != 6 || feb[0].InQty != 0 || feb[0].OutQty != 0 {
		t.Fatalf("2 月汇总期初应为 6，实际 %+v", feb)
	}

	janMoves, err := st.ListStockMoves(ctx, StockLedgerQuery{
		WarehouseID: wid, From: "2001-01-01", To: "2001-01-31", Page: 1, Size: 10,
	})
	if err != nil || janMoves.Total != 2 {
		t.Fatalf("1 月流水期望 2 条，实际 %d / %v", janMoves.Total, err)
	}
	febMoves, err := st.ListStockMoves(ctx, StockLedgerQuery{
		WarehouseID: wid, From: "2001-02-01", To: "2001-02-28", Page: 1, Size: 10,
	})
	if err != nil || febMoves.Total != 0 {
		t.Fatalf("2 月流水期望 0 条，实际 %d / %v", febMoves.Total, err)
	}
}
