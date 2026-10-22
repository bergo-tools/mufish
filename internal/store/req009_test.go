package store

import (
	"context"
	"errors"
	"testing"

	"mufish/internal/model"
)

// 余额调账必须生成独立记录，且编辑账户不能直接覆盖余额。
func TestAdjustAccountCreatesRecordAndEditCannotOverwriteBalance(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 7, Name: "财务"}

	accID, err := st.SaveAccount(ctx, 0, AccountInput{
		Code: "ADJ-A1", Name: "调账账户", Type: "cash",
		Balance: model.YuanFrom(100), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建账户失败: %v", err)
	}

	adj, err := st.AdjustAccount(ctx, accID, model.YuanFrom(250), "期初余额修正", user)
	if err != nil {
		t.Fatalf("调账失败: %v", err)
	}
	if adj.ID == 0 || adj.AccountID != accID || adj.AccountName != "调账账户" ||
		adj.BeforeBalance != model.YuanFrom(100) || adj.AfterBalance != model.YuanFrom(250) ||
		adj.Amount != model.YuanFrom(150) || adj.Reason != "期初余额修正" ||
		adj.CreatedBy != 7 || adj.CreatedByName != "财务" {
		t.Fatalf("调账记录不正确: %+v", adj)
	}

	acc, err := st.GetAccount(ctx, accID)
	if err != nil {
		t.Fatalf("读取账户失败: %v", err)
	}
	if acc.Balance != model.YuanFrom(250) {
		t.Fatalf("调账后余额期望 250，实际 %s", acc.Balance)
	}

	list, err := st.ListAccountAdjustments(ctx, accID, 10)
	if err != nil {
		t.Fatalf("查询调账记录失败: %v", err)
	}
	if len(list) != 1 || list[0].ID != adj.ID {
		t.Fatalf("调账记录列表不正确: %+v", list)
	}
	all, err := st.ListAccountAdjustments(ctx, 0, 10)
	if err != nil || len(all) != 1 {
		t.Fatalf("查询全部调账记录失败: %d / %v", len(all), err)
	}

	// 编辑账户时即使提交 balance 也不能覆盖余额。
	if _, err := st.SaveAccount(ctx, accID, AccountInput{
		Code: "ADJ-A1", Name: "调账账户（改）", Type: "cash",
		Balance: model.YuanFrom(999), Status: 1,
	}); err != nil {
		t.Fatalf("编辑账户失败: %v", err)
	}
	acc, _ = st.GetAccount(ctx, accID)
	if acc.Balance != model.YuanFrom(250) {
		t.Fatalf("编辑账户不应改变余额，期望 250，实际 %s", acc.Balance)
	}

	// 余额归零后仍不允许删除，避免调账记录失去归属。
	if _, err := st.AdjustAccount(ctx, accID, 0, "清零", user); err != nil {
		t.Fatalf("清零失败: %v", err)
	}
	if err := st.DeleteAccount(ctx, accID); !errors.Is(err, ErrInUse) {
		t.Fatalf("存在调账记录的账户应禁止删除，实际 %v", err)
	}
}

// 调账失败不能改变余额，也不能留下记录。
func TestAdjustAccountValidationAndRollback(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	user := OpUser{ID: 1, Name: "测试员"}

	accID, err := st.SaveAccount(ctx, 0, AccountInput{
		Code: "ADJ-A2", Name: "校验账户", Type: "cash",
		Balance: model.YuanFrom(100), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建账户失败: %v", err)
	}

	cases := []struct {
		name    string
		balance model.Money
		reason  string
	}{
		{"原因不能为空", model.YuanFrom(200), "   "},
		{"余额不能相同", model.YuanFrom(100), "相同余额"},
	}
	for _, tc := range cases {
		if _, err := st.AdjustAccount(ctx, accID, tc.balance, tc.reason, user); !errors.Is(err, ErrBadRequest) {
			t.Fatalf("%s 应返回参数错误，实际 %v", tc.name, err)
		}
	}
	if _, err := st.AdjustAccount(ctx, 999999, model.YuanFrom(200), "不存在", user); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在的账户应返回 ErrNotFound，实际 %v", err)
	}

	acc, _ := st.GetAccount(ctx, accID)
	if acc.Balance != model.YuanFrom(100) {
		t.Fatalf("失败调账不应改变余额，实际 %s", acc.Balance)
	}
	list, err := st.ListAccountAdjustments(ctx, accID, 10)
	if err != nil || len(list) != 0 {
		t.Fatalf("失败调账不应留下记录，实际 %d / %v", len(list), err)
	}
}

// 销售开单按仓库返回可用库存。
func TestAllProductsForWarehouseStock(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	pid, err := st.SaveProduct(ctx, 0, ProductInput{
		Code: "ADJ-P1", Name: "分仓库存商品", Unit: "个",
		PurchasePrice: model.PriceFrom(10), Status: 1,
	})
	if err != nil {
		t.Fatalf("创建商品失败: %v", err)
	}
	wa, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "ADJ-WA", Name: "A仓", Status: 1})
	if err != nil {
		t.Fatalf("创建 A 仓失败: %v", err)
	}
	wb, err := st.SaveWarehouse(ctx, 0, WarehouseInput{Code: "ADJ-WB", Name: "B仓", Status: 1})
	if err != nil {
		t.Fatalf("创建 B 仓失败: %v", err)
	}
	supplier, err := st.SavePartner(ctx, 0, PartnerInput{Code: "ADJ-S1", Name: "分仓供应商", Type: "supplier", Status: 1})
	if err != nil {
		t.Fatalf("创建供应商失败: %v", err)
	}
	buy(t, st, supplier, wa, pid, 7, 10)

	for _, tc := range []struct {
		warehouse int64
		want      model.Qty
	}{{wa, model.QtyFrom(7)}, {wb, model.QtyFrom(0)}} {
		list, err := st.AllProductsForWarehouse(ctx, "ADJ-P1", tc.warehouse)
		if err != nil {
			t.Fatalf("查询仓库 %d 商品失败: %v", tc.warehouse, err)
		}
		if len(list) != 1 || list[0].StockQty != tc.want {
			t.Fatalf("仓库 %d 可用库存期望 %s，实际 %+v", tc.warehouse, tc.want, list)
		}
	}
}
