package model

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestMoneyJSONAndScan(t *testing.T) {
	m := YuanFrom(12.345)
	if m != Money(1235) {
		t.Fatalf("12.345 元应四舍五入为 1235 分，实际 %d", m)
	}
	if m.String() != "12.35" {
		t.Fatalf("Money.String 期望 12.35，实际 %s", m)
	}
	if m.Float() != 12.35 {
		t.Fatalf("Money.Float 期望 12.35，实际 %v", m.Float())
	}

	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("序列化金额失败: %v", err)
	}
	if string(b) != "12.35" {
		t.Fatalf("金额 JSON 期望 12.35，实际 %s", b)
	}
	var back Money
	if err := json.Unmarshal([]byte("12.35"), &back); err != nil {
		t.Fatalf("反序列化金额失败: %v", err)
	}
	if back != m {
		t.Fatalf("金额反序列化期望 %d，实际 %d", m, back)
	}
	// 也接受字符串形式的数字。
	if err := json.Unmarshal([]byte("\"8.8\""), &back); err != nil {
		t.Fatalf("反序列化字符串金额失败: %v", err)
	}
	if back != Money(880) {
		t.Fatalf("字符串金额期望 880，实际 %d", back)
	}
	if err := json.Unmarshal([]byte("\"abc\""), &back); err == nil {
		t.Fatalf("非法金额应返回错误")
	}

	var scanned Money
	if err := scanned.Scan(nil); err != nil || scanned != 0 {
		t.Fatalf("nil 金额应扫描为 0，实际 %d / %v", scanned, err)
	}
	if err := scanned.Scan(float64(12.6)); err != nil || scanned != 13 {
		t.Fatalf("浮点金额应四舍五入，实际 %d / %v", scanned, err)
	}
	if err := scanned.Scan([]byte("42")); err != nil || scanned != 42 {
		t.Fatalf("字节金额扫描失败: %d / %v", scanned, err)
	}
	if err := scanned.Scan(struct{}{}); err == nil {
		t.Fatalf("非法金额类型应返回错误")
	}
}

func TestPriceJSONAndString(t *testing.T) {
	cases := []struct {
		value float64
		want  Price
		str   string
	}{
		{12, Price(120000), "12.00"},
		{12.5, Price(125000), "12.50"},
		{12.3456, Price(123456), "12.3456"},
		{12.34566, Price(123457), "12.3457"},
		{0, 0, "0.00"},
	}
	for _, c := range cases {
		p := PriceFrom(c.value)
		if p != c.want {
			t.Fatalf("PriceFrom(%v) 期望 %d，实际 %d", c.value, c.want, p)
		}
		if p.String() != c.str {
			t.Fatalf("Price(%v).String 期望 %s，实际 %s", c.value, c.str, p)
		}
	}
	b, err := json.Marshal(PriceFrom(12.5))
	if err != nil || string(b) != "12.50" {
		t.Fatalf("单价 JSON 期望 12.50，实际 %s / %v", b, err)
	}
	var back Price
	if err := json.Unmarshal([]byte("12.5"), &back); err != nil || back != PriceFrom(12.5) {
		t.Fatalf("单价反序列化失败: %d / %v", back, err)
	}
	if err := json.Unmarshal([]byte("null"), &back); err != nil || back != 0 {
		t.Fatalf("null 单价应反序列化为 0，实际 %d / %v", back, err)
	}
	if err := scannedPrice(); err != nil {
		t.Fatal(err)
	}
}

func scannedPrice() error {
	var p Price
	if err := p.Scan([]byte("12345")); err != nil {
		return err
	}
	if p != 12345 {
		return errors.New("单价扫描结果不正确")
	}
	return nil
}

func TestQtyJSONStringAndScan(t *testing.T) {
	cases := []struct {
		value float64
		want  Qty
		str   string
	}{
		{1.23456, Qty(1.2346), "1.2346"},
		{2, Qty(2), "2"},
		{0.00004, Qty(0), "0"},
		{1.5, Qty(1.5), "1.5"},
	}
	for _, c := range cases {
		q := QtyFrom(c.value)
		if q != c.want {
			t.Fatalf("QtyFrom(%v) 期望 %v，实际 %v", c.value, c.want, q)
		}
		if q.String() != c.str {
			t.Fatalf("Qty(%v).String 期望 %s，实际 %s", c.value, c.str, q)
		}
	}
	b, err := json.Marshal(QtyFrom(1.25))
	if err != nil || string(b) != "1.25" {
		t.Fatalf("数量 JSON 期望 1.25，实际 %s / %v", b, err)
	}
	var back Qty
	if err := json.Unmarshal([]byte("1.25"), &back); err != nil || back != QtyFrom(1.25) {
		t.Fatalf("数量反序列化失败: %v / %v", back, err)
	}

	var q Qty
	for _, src := range []any{nil, int64(3), float64(2.5), []byte("1.75"), "0.5"} {
		if err := q.Scan(src); err != nil {
			t.Fatalf("数量扫描 %v 失败: %v", src, err)
		}
	}
	if err := q.Scan([]byte("abc")); err == nil {
		t.Fatalf("非法字节数量应返回错误")
	}
	if err := q.Scan("abc"); err == nil {
		t.Fatalf("非法字符串数量应返回错误")
	}
	if err := q.Scan(struct{}{}); err == nil {
		t.Fatalf("非法数量类型应返回错误")
	}
}

func TestAmountOf(t *testing.T) {
	if got := AmountOf(QtyFrom(3), PriceFrom(0.3333)); got != YuanFrom(1) {
		t.Fatalf("0.3333 * 3 应四舍五入为 1.00 元，实际 %s", got)
	}
	if got := AmountOf(QtyFrom(1.5), PriceFrom(10)); got != YuanFrom(15) {
		t.Fatalf("1.5 * 10 期望 15.00 元，实际 %s", got)
	}
	if got := AmountOf(QtyFrom(0), PriceFrom(10)); got != 0 {
		t.Fatalf("数量为 0 时金额应为 0，实际 %s", got)
	}
	// 0.7 × 0.05 元 = 0.035 元，按分四舍五入应为 4 分；浮点计算会得到
	// 3.4999999999999996，从而错误地舍入为 3 分。
	if got := AmountOf(QtyFrom(0.7), PriceFrom(0.05)); got != Money(4) {
		t.Fatalf("0.7 × 0.05 应四舍五入为 4 分，实际 %d", got)
	}
	// 负数保持“远离零”的舍入规则。
	if got := AmountOf(QtyFrom(-0.7), PriceFrom(0.05)); got != Money(-4) {
		t.Fatalf("-0.7 × 0.05 应四舍五入为 -4 分，实际 %d", got)
	}
}

func TestNormDate(t *testing.T) {
	if got := NormDate(""); got != Today() {
		t.Fatalf("空日期应返回今天，实际 %s", got)
	}
	cases := map[string]string{
		"2026-01-02":                "2026-01-02",
		"2026/1/2":                  "2026-01-02",
		"2026-01-02 03:04:05":       "2026-01-02",
		"2026-01-02T03:04:05+08:00": "2026-01-02",
		"2026-01-02-extra":          "2026-01-02",
	}
	for in, want := range cases {
		if got := NormDate(in); got != want {
			t.Fatalf("NormDate(%q) 期望 %s，实际 %s", in, want, got)
		}
	}
}

func TestDocStatusAndBizTypeText(t *testing.T) {
	cases := map[int]string{0: "草稿", 1: "已确认", 2: "已作废", 9: "未知"}
	for in, want := range cases {
		if got := DocStatusText(in); got != want {
			t.Fatalf("DocStatusText(%d) 期望 %s，实际 %s", in, want, got)
		}
	}
	biz := map[string]string{
		"purchase": "采购入库", "sales": "销售出库", "take": "库存盘点",
		"transfer_in": "调拨入库", "transfer_out": "调拨出库", "init": "期初库存",
		"adjust": "手工调整", "other": "other",
	}
	for in, want := range biz {
		if got := BizTypeText(in); got != want {
			t.Fatalf("BizTypeText(%q) 期望 %s，实际 %s", in, want, got)
		}
	}
	if !strings.Contains(Today(), "-") {
		t.Fatalf("Today 格式异常: %s", Today())
	}
}
