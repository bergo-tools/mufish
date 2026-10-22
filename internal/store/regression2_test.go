package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"mufish/internal/model"
)

// 盘点单必须拒绝重复商品、负数实盘，并在账面库存变化后拒绝完成。
func TestStockTakeRejectsStaleBookAndInvalidItems(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)

	p, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(100), Price: model.PriceFrom(8)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}

	if _, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{
			{ProductID: pid, RealQty: model.QtyFrom(96)},
			{ProductID: pid, RealQty: model.QtyFrom(95)},
		},
	}, user); err == nil || !errors.Is(err, ErrBadRequest) {
		t.Fatalf("同一商品重复盘点应返回参数错误，实际 %v", err)
	}

	if _, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(-1)}},
	}, user); err == nil || !errors.Is(err, ErrBadRequest) {
		t.Fatalf("负数实盘数量应返回参数错误，实际 %v", err)
	}

	takeID, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(96)}},
	}, user)
	if err != nil {
		t.Fatalf("创建盘点单失败: %v", err)
	}

	// 盘点保存后库存发生变化，完成盘点应被拒绝而不是按旧差异调整。
	p2, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(10), Price: model.PriceFrom(8)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第二张采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p2, user); err != nil {
		t.Fatalf("确认第二张采购单失败: %v", err)
	}
	if err := st.FinishStockTake(ctx, takeID, user); err == nil || !errors.Is(err, ErrBadRequest) {
		t.Fatalf("账面库存变化后完成盘点应返回参数错误，实际 %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 110 {
		t.Fatalf("失败的盘点不应改变库存，期望 110，实际 %s", q)
	}

	// 按当前账面数重新盘点仍应正常工作。
	take2, err := st.SaveStockTake(ctx, 0, StockTakeInput{
		WarehouseID: wid, TakeDate: model.Today(),
		Items: []StockTakeItemInput{{ProductID: pid, RealQty: model.QtyFrom(108)}},
	}, user)
	if err != nil {
		t.Fatalf("创建第二张盘点单失败: %v", err)
	}
	if err := st.FinishStockTake(ctx, take2, user); err != nil {
		t.Fatalf("完成第二张盘点单失败: %v", err)
	}
	if q, _ := stockQty(t, st, wid, pid); q != 108 {
		t.Fatalf("盘点后期望 108，实际 %s", q)
	}
}

// 调拨确认后应按实际结转成本重新计算调拨金额。
func TestTransferConfirmRecalculatesTotal(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}
	pid, supplier, _, wid := bootstrap(t, st)

	p, err := st.SavePurchase(ctx, 0, PurchaseInput{
		SupplierID: supplier, WarehouseID: wid, OrderDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(100), Price: model.PriceFrom(8)}},
	}, user)
	if err != nil {
		t.Fatalf("创建采购单失败: %v", err)
	}
	if err := st.ConfirmPurchase(ctx, p, user); err != nil {
		t.Fatalf("确认采购单失败: %v", err)
	}
	w2, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "T-WH2", Name: "测试二仓", Status: 1})
	if err != nil {
		t.Fatalf("创建仓库失败: %v", err)
	}

	tr, err := st.SaveTransfer(ctx, 0, TransferInput{
		FromWarehouseID: wid, ToWarehouseID: w2, TransferDate: model.Today(),
		Items: []OrderItemInput{{ProductID: pid, Qty: model.QtyFrom(30), Price: 0}},
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
	if o.TotalAmount != model.YuanFrom(240) {
		t.Fatalf("调拨总额期望 240，实际 %s", o.TotalAmount)
	}
	if len(o.Items) != 1 || o.Items[0].Price != model.PriceFrom(8) {
		t.Fatalf("调拨明细成本单价期望 8，实际 %+v", o.Items)
	}
}

// 从未入库、没有 stocks 记录的商品也必须出现在库存预警中。
func TestLowStockIncludesZeroStockProducts(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	pid, err := st.SaveProduct(ctx, 0, ProductInput{
		Code: "LOW-P1", Name: "零库存预警商品", Unit: "个",
		SafetyStock: model.QtyFrom(5), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	wh, err := st.DefaultWarehouse(ctx)
	if err != nil {
		t.Fatalf("读取默认仓库失败: %v", err)
	}

	all, err := st.LowStockList(ctx, 0, 10)
	if err != nil {
		t.Fatalf("查询全部仓库预警失败: %v", err)
	}
	var found *model.Stock
	for i := range all {
		if all[i].ProductID == pid {
			found = &all[i]
		}
	}
	if found == nil {
		t.Fatalf("零库存商品应出现在预警列表中: %+v", all)
	}
	if found.Qty != 0 || found.WarehouseName != "全部仓库" {
		t.Fatalf("零库存预警数据不正确: %+v", found)
	}

	one, err := st.LowStockList(ctx, wh.ID, 10)
	if err != nil {
		t.Fatalf("查询单仓预警失败: %v", err)
	}
	found = nil
	for i := range one {
		if one[i].ProductID == pid {
			found = &one[i]
		}
	}
	if found == nil {
		t.Fatalf("零库存商品应出现在单仓预警列表中: %+v", one)
	}
	if found.Qty != 0 || found.WarehouseName != wh.Name {
		t.Fatalf("单仓零库存预警数据不正确: %+v", found)
	}

	// 库存查询的低库存筛选必须与预警使用同一口径。
	allPage, err := st.ListStocks(ctx, StockQuery{OnlyLow: true, Page: 1, Size: 100})
	if err != nil {
		t.Fatalf("查询全部仓库低库存失败: %v", err)
	}
	if allPage.Total != int64(len(all)) {
		t.Fatalf("全部仓库低库存口径不一致：查询 %d 条，预警 %d 条", allPage.Total, len(all))
	}
	onePage, err := st.ListStocks(ctx, StockQuery{WarehouseID: wh.ID, OnlyLow: true, Page: 1, Size: 100})
	if err != nil {
		t.Fatalf("查询单仓低库存失败: %v", err)
	}
	if onePage.Total != int64(len(one)) {
		t.Fatalf("单仓低库存口径不一致：查询 %d 条，预警 %d 条", onePage.Total, len(one))
	}
}

// 恢复无效备份文件时必须回滚，不能让 Store 停留在已关闭状态。
func TestRestoreFromInvalidFileRollsBack(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	bad := filepath.Join(t.TempDir(), "bad.db")
	if err := os.WriteFile(bad, []byte("this is not a sqlite database"), 0o600); err != nil {
		t.Fatalf("写入无效备份文件失败: %v", err)
	}
	if err := st.RestoreFrom(ctx, bad); err == nil {
		t.Fatalf("恢复无效数据库应失败")
	}
	// 回滚后原有数据仍应可读。
	products, err := st.ListProducts(ctx, ProductQuery{Status: -1, Page: 1, Size: 10})
	if err != nil {
		t.Fatalf("回滚后查询商品失败: %v", err)
	}
	if products.Total == 0 {
		t.Fatalf("回滚后应能读到原有商品数据")
	}
}
