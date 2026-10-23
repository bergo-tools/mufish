package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// 金额 / 单价 / 数量
//
// 为避免浮点误差，数据库中金额一律以「分」为单位的整数保存，单价以「万分之一元」
// 为单位的整数保存，数量使用 REAL（最多 4 位小数）。
// 对前端（JSON）暴露时统一转换为「元」，因此业务代码只需操作这些类型。
// ---------------------------------------------------------------------------

// Money 金额，单位为分（0.01 元）。
type Money int64

// YuanFrom 由「元」构造金额。
func YuanFrom(v float64) Money { return Money(math.Round(v * 100)) }

// Float 返回以元为单位的浮点值。
func (m Money) Float() float64 { return float64(m) / 100 }

// String 返回保留两位小数的元字符串。
func (m Money) String() string { return strconv.FormatFloat(m.Float(), 'f', 2, 64) }

func (m Money) MarshalJSON() ([]byte, error) { return []byte(m.String()), nil }

func (m *Money) UnmarshalJSON(b []byte) error {
	f, err := parseJSONNumber(b)
	if err != nil {
		return fmt.Errorf("金额格式错误: %w", err)
	}
	*m = YuanFrom(f)
	return nil
}

func (m *Money) Scan(src any) error {
	v, err := scanInt(src)
	if err != nil {
		return err
	}
	*m = Money(v)
	return nil
}

func (m Money) Value() (driver.Value, error) { return int64(m), nil }

// Price 单价，单位为万分之一元（0.0001 元），可表示 4 位小数单价。
type Price int64

// PriceFrom 由「元」构造单价。
func PriceFrom(v float64) Price { return Price(math.Round(v * 10000)) }

// Float 返回以元为单位的浮点值。
func (p Price) Float() float64 { return float64(p) / 10000 }

// String 返回最多 4 位小数、至少 2 位小数的元字符串。
func (p Price) String() string { return formatTrim(p.Float(), 4, 2) }

func (p Price) MarshalJSON() ([]byte, error) { return []byte(p.String()), nil }

func (p *Price) UnmarshalJSON(b []byte) error {
	f, err := parseJSONNumber(b)
	if err != nil {
		return fmt.Errorf("单价格式错误: %w", err)
	}
	*p = PriceFrom(f)
	return nil
}

func (p *Price) Scan(src any) error {
	v, err := scanInt(src)
	if err != nil {
		return err
	}
	*p = Price(v)
	return nil
}

func (p Price) Value() (driver.Value, error) { return int64(p), nil }

// AmountOf 按「数量 × 单价」计算金额（四舍五入到分）。
//
// 数量按 4 位小数、单价按万分之一元换算成整数后再计算，避免
// 0.7 × 0.05 这类刚好半分的组合被浮点误差舍入到错误方向。
// 金额（分）= round(q4 × p4 / 1,000,000)，负数按远离零舍入。
func AmountOf(qty Qty, price Price) Money {
	q4 := int64(math.Round(float64(qty) * 10000))
	p4 := int64(price)
	if q4 == 0 || p4 == 0 {
		return 0
	}

	// 使用大整数避免 q4 × p4 在极端输入下溢出；日常金额不会走到
	// 这个规模，但金额计算不应默默回绕。
	var product, factor big.Int
	product.SetInt64(q4)
	factor.SetInt64(p4)
	product.Mul(&product, &factor)
	if product.Sign() >= 0 {
		product.Add(&product, big.NewInt(500000))
	} else {
		product.Sub(&product, big.NewInt(500000))
	}
	product.Quo(&product, big.NewInt(1000000))
	return Money(product.Int64())
}

// Qty 数量，最多 4 位小数。
type Qty float64

// QtyFrom 由浮点构造数量。
func QtyFrom(v float64) Qty { return Qty(roundTo(v, 4)) }

func (q Qty) Float() float64 { return float64(q) }

func (q Qty) String() string { return formatTrim(float64(q), 4, 0) }

func (q Qty) MarshalJSON() ([]byte, error) { return []byte(q.String()), nil }

func (q *Qty) UnmarshalJSON(b []byte) error {
	f, err := parseJSONNumber(b)
	if err != nil {
		return fmt.Errorf("数量格式错误: %w", err)
	}
	*q = QtyFrom(f)
	return nil
}

func (q *Qty) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*q = 0
	case int64:
		*q = Qty(v)
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("无法解析数量: 非有限数值 %v", v)
		}
		*q = Qty(v)
	case []byte:
		f, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64)
		if err != nil {
			return fmt.Errorf("无法解析数量 %q: %w", string(v), err)
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("无法解析数量 %q: 非有限数值", string(v))
		}
		*q = Qty(f)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return fmt.Errorf("无法解析数量 %q: %w", v, err)
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("无法解析数量 %q: 非有限数值", v)
		}
		*q = Qty(f)
	default:
		return fmt.Errorf("无法解析数量: %v", src)
	}
	return nil
}

func (q Qty) Value() (driver.Value, error) { return float64(q), nil }

func roundTo(v float64, digits int) float64 {
	p := math.Pow10(digits)
	return math.Round(v*p) / p
}

// formatTrim 四舍五入到 maxDigits 位，去掉多余的 0，但至少保留 minDigits 位。
func formatTrim(v float64, maxDigits, minDigits int) string {
	v = roundTo(v, maxDigits)
	s := strconv.FormatFloat(v, 'f', -1, 64)
	dot := strings.IndexByte(s, '.')
	if dot < 0 {
		if minDigits > 0 {
			return s + "." + strings.Repeat("0", minDigits)
		}
		return s
	}
	dec := len(s) - dot - 1
	if dec > maxDigits {
		s = strconv.FormatFloat(roundTo(v, maxDigits), 'f', maxDigits, 64)
		dec = maxDigits
	}
	if dec < minDigits {
		return s + strings.Repeat("0", minDigits-dec)
	}
	return s
}

func parseJSONNumber(b []byte) (float64, error) {
	s := strings.TrimSpace(string(b))
	if s == "" || s == "null" {
		return 0, nil
	}
	s = strings.Trim(s, `"`)
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	// encoding/json 会拒绝 NaN / Inf 字面量，但自定义 Unmarshal 还接受
	// 字符串形式；若不在这里拦截，非法值会绕过后续的 <0 等业务校验。
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("不是有效数字（%q）", s)
	}
	return f, nil
}

func scanInt(src any) (int64, error) {
	switch v := src.(type) {
	case nil:
		return 0, nil
	case int64:
		return v, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, fmt.Errorf("无法解析整数: 非有限数值 %v", v)
		}
		return int64(math.Round(v)), nil
	case []byte:
		if len(v) == 0 {
			return 0, nil
		}
		return strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64)
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, nil
		}
		return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	default:
		return 0, fmt.Errorf("无法解析整数: %v", src)
	}
}

// ---------------------------------------------------------------------------
// 时间助手
// ---------------------------------------------------------------------------

// TimeLayout 数据库中统一使用的日期时间格式（本地时间）。
const TimeLayout = "2006-01-02 15:04:05"

// DateLayout 数据库中统一使用的日期格式。
const DateLayout = "2006-01-02"

// Now 返回当前时间的存储字符串。
func Now() string { return time.Now().Format(TimeLayout) }

// Today 返回今天的日期字符串。
func Today() string { return time.Now().Format(DateLayout) }

// NormDate 规范化用户输入的日期。
func NormDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return Today()
	}
	for _, layout := range []string{DateLayout, TimeLayout, time.RFC3339, "2006/01/02", "2006/1/2"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t.Format(DateLayout)
		}
	}
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// ValidDate 判断字符串是否为 YYYY-MM-DD 格式的有效日期。
func ValidDate(s string) bool {
	_, err := time.ParseInLocation(DateLayout, s, time.Local)
	return err == nil
}

// JSONNumber 把数值原样输出（用于自定义 Marshal）。
func JSONNumber(v float64) json.RawMessage {
	return json.RawMessage(strconv.FormatFloat(v, 'f', -1, 64))
}
