package statistic

import (
	"context"
	"fmt"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/pkg/constants"
	"ledger/src/slog"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/shopspring/decimal"
)

type trendAcc struct {
	Income  decimal.Decimal
	Expense decimal.Decimal
}

// aggregateTrend 按 layout 归桶累加，key 使用 time.Local
func aggregateTrend(txs []*models.TrendTx, layout string) map[string]*trendAcc {
	stat := make(map[string]*trendAcc)
	for _, tx := range txs {
		key := time.UnixMilli(tx.OccAt).In(time.Local).Format(layout)
		a := stat[key]
		if a == nil {
			a = &trendAcc{}
			stat[key] = a
		}
		if tx.Amount.IsPositive() {
			a.Income = a.Income.Add(tx.Amount)
		} else {
			a.Expense = a.Expense.Add(tx.Amount.Abs())
		}
	}
	return stat
}

// fillTrend 从 fromMs 到 toMs（不含）按粒度补零，layout 与 step 对应
func fillTrend(fromMs, toMs int64, layout string, step func(time.Time) time.Time, stat map[string]*trendAcc) []map[string]string {
	var fullList []map[string]string
	current := time.UnixMilli(fromMs).In(time.Local)
	endTime := time.UnixMilli(toMs).In(time.Local)

	for current.Before(endTime) {
		key := current.Format(layout)
		inc, exp := decimal.Zero, decimal.Zero
		if a, ok := stat[key]; ok {
			inc, exp = a.Income, a.Expense
		}
		fullList = append(fullList, map[string]string{
			"day":     key,
			"income":  inc.Round(2).StringFixed(2),
			"expense": exp.Round(2).StringFixed(2),
		})
		current = step(current)
	}
	return fullList
}

// DayTrend 日趋势
func DayTrend() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		var req QueryArgs
		if err := c.BindAndValidate(&req); err != nil {
			klog.Error("BindAndValidate ", err)
			c.JSON(http.StatusBadRequest, answer.ResBody(answer.EcodeInvalidRequestError, err.Error(), nil))
			return
		}

		from, to, name, dayCnt, err := ResolveCycleRange(req.Ct, req.Td, req.From, req.To)
		if err != nil {
			klog.Error("ResolveCycleRange err", err)
			c.JSON(http.StatusBadRequest, answer.ResBody(answer.EcodeInvalidRequestError, "时间参数错误", nil))
			return
		}
		fmt.Println(from)
		fmt.Println(to)
		fmt.Println(name)
		fmt.Println(dayCnt)

		dao, err := models.NewDBModel(c.GetString(constants.InstanceIdKey))
		if err != nil {
			klog.Errorf("NewDBModel error %s", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, err.Error(), nil))
			return
		}

		txs, err := dao.TrendTxs(from, to)
		if err != nil {
			klog.Errorf("TrendTxs error %v", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, "统计失败", nil))
			return
		}

		stat := aggregateTrend(txs, "2006-01-02")
		list := fillTrend(from, to, "2006-01-02", func(t time.Time) time.Time {
			return t.AddDate(0, 0, 1)
		}, stat)

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, list))
	}
}

// MonthTrend 按月统计，补齐空白月份
func MonthTrend() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		var req QueryArgs
		if err := c.BindAndValidate(&req); err != nil {
			klog.Error("BindAndValidate ", err)
			c.JSON(http.StatusBadRequest, answer.ResBody(answer.EcodeInvalidRequestError, err.Error(), nil))
			return
		}

		from, to, name, dayCnt, err := ResolveCycleRange(req.Ct, req.Td, req.From, req.To)
		if err != nil {
			klog.Error("ResolveCycleRange err", err)
			c.JSON(http.StatusBadRequest, answer.ResBody(answer.EcodeInvalidRequestError, "时间参数错误", nil))
			return
		}
		fmt.Println(from)
		fmt.Println(to)
		fmt.Println(name)
		fmt.Println(dayCnt)

		dao, err := models.NewDBModel(c.GetString(constants.InstanceIdKey))
		if err != nil {
			klog.Errorf("NewDBModel error %s", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, err.Error(), nil))
			return
		}

		txs, err := dao.TrendTxs(from, to)
		if err != nil {
			klog.Errorf("TrendTxs error %v", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, "统计失败", nil))
			return
		}

		stat := aggregateTrend(txs, "2006-01")
		list := fillTrend(from, to, "2006-01", func(t time.Time) time.Time {
			return t.AddDate(0, 1, 0)
		}, stat)

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, list))
	}
}

// YearTrend 按年统计，补齐空白年份
func YearTrend() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		var req QueryArgs
		if err := c.BindAndValidate(&req); err != nil {
			klog.Error("BindAndValidate ", err)
			c.JSON(http.StatusBadRequest, answer.ResBody(answer.EcodeInvalidRequestError, err.Error(), nil))
			return
		}

		from, to, name, dayCnt, err := ResolveCycleRange(req.Ct, req.Td, req.From, req.To)
		if err != nil {
			klog.Error("ResolveCycleRange err", err)
			c.JSON(http.StatusBadRequest, answer.ResBody(answer.EcodeInvalidRequestError, "时间参数错误", nil))
			return
		}
		fmt.Println(from)
		fmt.Println(to)
		fmt.Println(name)
		fmt.Println(dayCnt)

		dao, err := models.NewDBModel(c.GetString(constants.InstanceIdKey))
		if err != nil {
			klog.Errorf("NewDBModel error %s", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, err.Error(), nil))
			return
		}

		txs, err := dao.TrendTxs(from, to)
		if err != nil {
			klog.Errorf("TrendTxs error %v", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, "统计失败", nil))
			return
		}

		stat := aggregateTrend(txs, "2006")
		list := fillTrend(from, to, "2006", func(t time.Time) time.Time {
			return t.AddDate(1, 0, 0)
		}, stat)

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, list))
	}
}
