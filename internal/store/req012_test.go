package store

import (
	"context"
	"errors"
	"testing"

	"mufish/internal/model"
)

// 纯供应商/纯客户的期初余额必须与类型匹配，否则首页合计与应收应付
// 报表会一个包含、一个排除，用户无法对账。
func TestPartnerInitialBalanceMustMatchType(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	cases := []struct {
		name string
		in   PartnerInput
	}{
		{
			name: "纯供应商不能填写期初应收",
			in: PartnerInput{
				Code: "REQ012-S1", Name: "错误供应商", Type: "supplier",
				InitReceivable: model.YuanFrom(100), Status: 1,
			},
		},
		{
			name: "纯客户不能填写期初应付",
			in: PartnerInput{
				Code: "REQ012-C1", Name: "错误客户", Type: "customer",
				InitPayable: model.YuanFrom(100), Status: 1,
			},
		},
	}
	for _, tc := range cases {
		if _, err := st.SavePartner(ctx, 0, tc.in); !errors.Is(err, ErrBadRequest) {
			t.Fatalf("%s 应返回 ErrBadRequest，实际 %v", tc.name, err)
		}
		var count int
		if err := st.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM partners WHERE code=?", tc.in.Code).Scan(&count); err != nil {
			t.Fatalf("%s 查询失败: %v", tc.name, err)
		}
		if count != 0 {
			t.Fatalf("%s 失败后不应写入往来单位，实际 %d 条", tc.name, count)
		}
	}

	// 客户兼供应商可以同时填写期初应收和期初应付。
	if _, err := st.SavePartner(ctx, 0, PartnerInput{
		Code: "REQ012-B1", Name: "客户兼供应商", Type: "both",
		InitReceivable: model.YuanFrom(30), InitPayable: model.YuanFrom(20), Status: 1,
	}); err != nil {
		t.Fatalf("客户兼供应商应允许同时填写期初余额: %v", err)
	}
}

// 首页和列表必须与应收应付报表采用相同的类型口径。
func TestDashboardAndPartnerListExcludeMismatchedInitialBalance(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// 直接写入历史脏数据：纯供应商带期初应收、纯客户带期初应付。
	if _, err := st.db.ExecContext(ctx,
		"INSERT INTO partners(code,name,type,init_receivable,init_payable,status,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)",
		"REQ012-LEGACY-S", "历史供应商", "supplier", int64(model.YuanFrom(70)), int64(0),
		model.Now(), model.Now()); err != nil {
		t.Fatalf("写入历史供应商失败: %v", err)
	}
	if _, err := st.db.ExecContext(ctx,
		"INSERT INTO partners(code,name,type,init_receivable,init_payable,status,created_at,updated_at) VALUES(?,?,?,?,?,1,?,?)",
		"REQ012-LEGACY-C", "历史客户", "customer", int64(0), int64(model.YuanFrom(80)),
		model.Now(), model.Now()); err != nil {
		t.Fatalf("写入历史客户失败: %v", err)
	}

	d, err := st.Dashboard(ctx)
	if err != nil {
		t.Fatalf("查询看板失败: %v", err)
	}
	if d.Receivable != 0 || d.Payable != 0 {
		t.Fatalf("首页不应计入与单位类型不匹配的期初余额，应收 %s 应付 %s", d.Receivable, d.Payable)
	}

	page, err := st.ListPartners(ctx, PartnerQuery{Page: 1, Size: 100})
	if err != nil {
		t.Fatalf("查询往来单位失败: %v", err)
	}
	for _, p := range page.List {
		switch p.Code {
		case "REQ012-LEGACY-S":
			if p.Receivable != 0 {
				t.Fatalf("纯供应商的应收应为 0，实际 %s", p.Receivable)
			}
		case "REQ012-LEGACY-C":
			if p.Payable != 0 {
				t.Fatalf("纯客户的应付应为 0，实际 %s", p.Payable)
			}
		}
	}
}
