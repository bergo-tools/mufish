package store

import (
	"context"
	"testing"

	"mufish/internal/model"
)

// P0-1：盘盈入库后又有其他入库时，作废盘点必须按原调整金额冲减。
func TestP0VoidStockTakeUsesOriginalAmount(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)
	buy(t, st, supplier, wid, pid, 10, 10)

	takeID, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(12)}},
	}, user)
	if err != nil {
		t.Fatalf("创建盘点单失败: %v", err)
	}
	if err := st.FinishStockTake(ctx, takeID, user); err != nil {
		t.Fatalf("完成盘点失败: %v", err)
	}
	buy(t, st, supplier, wid, pid, 2, 20)

	qty, amount, price := stockState(t, st, wid, pid)
	if qty != 14 || amount != int64(model.YuanFrom(160)) || price != int64(model.PriceFrom(11.4286)) {
		t.Fatalf("作废前库存应为 14 个 / 160 元 / 均价 11.4286 元，实际 %.4f / %d / %d", qty, amount, price)
	}

	if err := st.VoidStockTake(ctx, takeID, user); err != nil {
		t.Fatalf("作废盘点失败: %v", err)
	}
	qty, amount, price = stockState(t, st, wid, pid)
	if qty != 12 {
		t.Fatalf("作废盘点后数量应为 12，实际 %.4f", qty)
	}
	if amount != int64(model.YuanFrom(140)) {
		t.Fatalf("作废盘点后结存金额应为 140 元，实际 %s（按当前均价冲减会得到约 137.14 元）", model.Money(amount))
	}
	if price != int64(model.PriceFrom(11.6667)) {
		t.Fatalf("作废盘点后结存单价应为 11.6667 元，实际 %s", model.Price(price))
	}
}
