package statistic

import (
	"testing"
	"time"

	"ledger/src/models"

	"github.com/shopspring/decimal"
)

// localMs 按本机时区构造毫秒时间戳
func localMs(s string) int64 {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t.UnixMilli()
}

// dec 从字符串构造 decimal
func dec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

// 模拟明细：含跨零点、跨月、跨年、收入/支出混合
func mockTrendTxs() []*models.TrendTx {
	return []*models.TrendTx{
		{OccAt: localMs("2026-11-10 22:41:18"), Amount: dec("-24.90")},    // 育婴
		{OccAt: localMs("2026-11-12 17:30:04"), Amount: dec("-400.00")},   // 医疗
		{OccAt: localMs("2026-11-12 17:31:44"), Amount: dec("-400.00")},   // 医疗
		{OccAt: localMs("2026-11-13 07:59:26"), Amount: dec("-55555.00")}, // ★跨日关键用例
		{OccAt: localMs("2026-11-13 08:00:04"), Amount: dec("-10000.00")}, // 与上一笔同天
		{OccAt: localMs("2026-11-30 23:59:59"), Amount: dec("8888.00")},   // 月末最后一秒，收入
		{OccAt: localMs("2026-12-01 00:00:00"), Amount: dec("-1.00")},     // 12月第一毫秒
		{OccAt: localMs("2025-12-31 23:59:59"), Amount: dec("-5.00")},     // 上年最后一秒
		{OccAt: localMs("2026-01-01 00:00:00"), Amount: dec("-5.00")},     // 本年第一毫秒
	}
}

// TestAggregateTrend_Day_CrossMidnight 核心：07:59:26 必须归 11-13
func TestAggregateTrend_Day_CrossMidnight(t *testing.T) {
	stat := aggregateTrend(mockTrendTxs(), "2006-01-02")

	// 11-13 包含 55555 + 10000
	if a := stat["2026-11-13"]; a == nil || !a.Expense.Equal(dec("65555.00")) {
		t.Fatalf("2026-11-13 expense = %v, want 65555.00", a)
	}
	// 11-12 只有两笔医疗
	if a := stat["2026-11-12"]; a == nil || !a.Expense.Equal(dec("800.00")) {
		t.Fatalf("2026-11-12 expense = %v, want 800.00", a)
	}
	// 11-30 有收入
	if a := stat["2026-11-30"]; a == nil || !a.Income.Equal(dec("8888.00")) {
		t.Fatalf("2026-11-30 income = %v, want 8888.00", a)
	}
	// 12-01 单独成天
	if a := stat["2026-12-01"]; a == nil || !a.Expense.Equal(dec("1.00")) {
		t.Fatalf("2026-12-01 expense = %v, want 1.00", a)
	}
	// 07:59 那笔不能出现在 11-12
	if a := stat["2026-11-12"]; a != nil && a.Expense.Equal(dec("56355.00")) {
		t.Fatalf("55555 was wrongly grouped into 11-12")
	}
}

// TestAggregateTrend_Month 跨月归桶
func TestAggregateTrend_Month(t *testing.T) {
	stat := aggregateTrend(mockTrendTxs(), "2006-01")

	// 2026-11: 支出 24.90+400+400+55555+10000 = 66379.90，收入 8888
	if a := stat["2026-11"]; a == nil ||
		!a.Expense.Equal(dec("66379.90")) || !a.Income.Equal(dec("8888.00")) {
		t.Fatalf("2026-11 = %+v", a)
	}
	// 2026-12: 支出 1
	if a := stat["2026-12"]; a == nil || !a.Expense.Equal(dec("1.00")) {
		t.Fatalf("2026-12 = %+v", a)
	}
	// 2025-12: 支出 5（上年最后一秒）
	if a := stat["2025-12"]; a == nil || !a.Expense.Equal(dec("5.00")) {
		t.Fatalf("2025-12 = %+v", a)
	}
}

// TestAggregateTrend_Year 跨年归桶
func TestAggregateTrend_Year(t *testing.T) {
	stat := aggregateTrend(mockTrendTxs(), "2006")

	if a := stat["2025"]; a == nil || !a.Expense.Equal(dec("5.00")) {
		t.Fatalf("2025 = %+v", a)
	}
	// 2026: 11月支出 66379.90 + 12月 1 + 1月 5 = 66385.90，收入 8888
	if a := stat["2026"]; a == nil ||
		!a.Expense.Equal(dec("66385.90")) || !a.Income.Equal(dec("8888.00")) {
		t.Fatalf("2026 = %+v", a)
	}
}

// TestFillTrend_Day_Continuous 补零连续 + 起止 key 正确
func TestFillTrend_Day_Continuous(t *testing.T) {
	stat := aggregateTrend(mockTrendTxs(), "2006-01-02")

	from := localMs("2026-11-29 00:00:00")
	to := localMs("2026-12-02 00:00:00") // 不含
	list := fillTrend(from, to, "2006-01-02", func(tm time.Time) time.Time {
		return tm.AddDate(0, 0, 1)
	}, stat)

	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	wantKeys := []string{"2026-11-29", "2026-11-30", "2026-12-01"}
	for i, k := range wantKeys {
		if list[i]["day"] != k {
			t.Fatalf("list[%d].day = %s, want %s", i, list[i]["day"], k)
		}
	}
	// 11-29 无交易补 0
	if list[0]["income"] != "0.00" || list[0]["expense"] != "0.00" {
		t.Fatalf("11-29 should be zero, got %+v", list[0])
	}
	// 11-30 有收入
	if list[1]["income"] != "8888.00" {
		t.Fatalf("11-30 income = %s", list[1]["income"])
	}
}

// TestFillTrend_Month_Continuous 按月补零
func TestFillTrend_Month_Continuous(t *testing.T) {
	stat := aggregateTrend(mockTrendTxs(), "2006-01")

	from := localMs("2025-12-01 00:00:00")
	to := localMs("2027-01-01 00:00:00") // 不含，覆盖 2025-12 ~ 2026-12 共 13 个月
	list := fillTrend(from, to, "2006-01", func(tm time.Time) time.Time {
		return tm.AddDate(0, 1, 0)
	}, stat)

	if len(list) != 13 {
		t.Fatalf("len = %d, want 13", len(list))
	}
	if list[0]["day"] != "2025-12" || list[len(list)-1]["day"] != "2026-12" {
		t.Fatalf("range wrong: %s ~ %s", list[0]["day"], list[len(list)-1]["day"])
	}
	// 中间无交易月份补 0
	for _, p := range list {
		if p["day"] == "2026-06" && (p["income"] != "0.00" || p["expense"] != "0.00") {
			t.Fatalf("2026-06 should be zero, got %+v", p)
		}
	}
}

// TestFillTrend_Year_Continuous 按年补零
func TestFillTrend_Year_Continuous(t *testing.T) {
	stat := aggregateTrend(mockTrendTxs(), "2006")

	from := localMs("2025-01-01 00:00:00")
	to := localMs("2027-01-01 00:00:00") // 不含，覆盖 2025、2026
	list := fillTrend(from, to, "2006", func(tm time.Time) time.Time {
		return tm.AddDate(1, 0, 0)
	}, stat)

	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0]["day"] != "2025" || list[1]["day"] != "2026" {
		t.Fatalf("keys = %s, %s", list[0]["day"], list[1]["day"])
	}
	if list[0]["expense"] != "5.00" {
		t.Fatalf("2025 expense = %s", list[0]["expense"])
	}
}

// TestFillTrend_Empty 空数据补零
func TestFillTrend_Empty(t *testing.T) {
	stat := aggregateTrend(nil, "2006-01-02")
	from := localMs("2026-11-01 00:00:00")
	to := localMs("2026-11-04 00:00:00")
	list := fillTrend(from, to, "2006-01-02", func(tm time.Time) time.Time {
		return tm.AddDate(0, 0, 1)
	}, stat)

	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	for _, p := range list {
		if p["income"] != "0.00" || p["expense"] != "0.00" {
			t.Fatalf("should be zero: %+v", p)
		}
	}
}

// TestAggregateTrend_ExpensePositive 支出取绝对值、保留两位
func TestAggregateTrend_ExpensePositive(t *testing.T) {
	txs := []*models.TrendTx{
		{OccAt: localMs("2026-11-13 10:00:00"), Amount: dec("-123.45")},
	}
	stat := aggregateTrend(txs, "2006-01-02")
	a := stat["2026-11-13"]
	if a == nil || !a.Expense.Equal(dec("123.45")) {
		t.Fatalf("expense = %v, want 123.45", a)
	}
	if a.Income.IsPositive() {
		t.Fatalf("income should be zero")
	}
}

// TestAggregateTrend_DecimalPrecision 浮点累加不应丢精度
func TestAggregateTrend_DecimalPrecision(t *testing.T) {
	txs := []*models.TrendTx{
		{OccAt: localMs("2026-11-13 10:00:00"), Amount: dec("0.10")},
		{OccAt: localMs("2026-11-13 10:00:01"), Amount: dec("0.20")},
	}
	stat := aggregateTrend(txs, "2006-01-02")
	if !stat["2026-11-13"].Income.Equal(dec("0.30")) {
		t.Fatalf("income = %v, want 0.30", stat["2026-11-13"].Income)
	}
}

// TestFillTrend_KeyMatchAggregate 补零 key 与聚合 key 口径一致（时区不漂）
func TestFillTrend_KeyMatchAggregate(t *testing.T) {
	txs := []*models.TrendTx{
		{OccAt: localMs("2026-11-13 07:59:26"), Amount: dec("-55555.00")},
	}
	stat := aggregateTrend(txs, "2006-01-02")

	from := localMs("2026-11-13 00:00:00")
	to := localMs("2026-11-14 00:00:00")
	list := fillTrend(from, to, "2006-01-02", func(tm time.Time) time.Time {
		return tm.AddDate(0, 0, 1)
	}, stat)

	if len(list) != 1 {
		t.Fatalf("len = %d, want 1", len(list))
	}
	if list[0]["day"] != "2026-11-13" || list[0]["expense"] != "55555.00" {
		t.Fatalf("key mismatch: %+v", list[0])
	}
}
