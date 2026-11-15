package statistic

import (
	"context"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/pkg/constants"
	"ledger/src/slog"
	"net/http"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/shopspring/decimal"
)

// StatOve 数据总览
func StatOve() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		sa := c.Query("sa") // 总览类型
		pt := c.Query("pt") // 统计周期

		from := c.Query("from")
		to := c.Query("to")

		switch sa {
		case "s01": // 统计收支
			S01(c, pt, from, to)
		case "s02": // 分类统计
			// ...
		default:
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeQueryError, "invalid overview type", nil))
		}
	}
}

func S01(c *app.RequestContext, pt, from, to string) {
	klog := slog.FromContext(c)

	if from == "" {
		now := time.Now()
		switch pt {
		case "year":
			from = now.Format("2006")
		case "month":
			from = now.Format("2006-01")
		case "custom":
			from = now.Format("2006-01-02")
		default:
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeQueryError, "unsupported period type", nil))
			return
		}
	}
	if pt == "custom" && to == "" {
		to = from
	}

	var (
		startMs, endMs int64
		err            error
	)

	// 关键：ct/td 与 from/to 二选一，不能同时传
	switch pt {
	case "year":
		startMs, endMs, _, _, err = ResolveCycleRange("year", from, "", "")
	case "month":
		startMs, endMs, _, _, err = ResolveCycleRange("month", from, "", "")
	case "custom":
		startMs, endMs, _, _, err = ResolveCycleRange("", "", from, to)
	default:
		c.JSON(http.StatusBadRequest,
			answer.ResBody(answer.EcodeQueryError, "unsupported period type", nil))
		return
	}

	if err != nil {
		klog.Warnf("ResolveCycleRange err %v", err)
		c.JSON(http.StatusBadRequest,
			answer.ResBody(answer.EcodeInvalidTimeRange, "invalid time range", nil))
		return
	}

	dao, err := models.NewDBModel(c.GetString(constants.InstanceIdKey))
	if err != nil {
		klog.Errorf("NewDBModel error %s", err)
		c.JSON(http.StatusInternalServerError,
			answer.ResBody(answer.EcodeError, "internal error", nil))
		return
	}

	result, err := dao.CountSouZhi(startMs, endMs)
	if err != nil {
		klog.Errorf("统计收支失败: %v", err)
		c.JSON(http.StatusInternalServerError,
			answer.ResBody(answer.EcodeError, "statistic failed", nil))
		return
	}

	type S01StatResp struct {
		Income  decimal.Decimal `json:"income"`
		Expense decimal.Decimal `json:"expense"`
	}
	payload := S01StatResp{
		Income:  result.Income,
		Expense: result.Expense,
	}

	c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, payload))
}

// CategoryPie 收支分类饼图
func CategoryPie() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		var req QueryArgs
		if err := c.BindAndValidate(&req); err != nil {
			klog.Error("BindAndValidate ", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeQueryError, "invalid query parameter", nil))
			return
		}
		from, to, _, _, err := ResolveCycleRange(req.Ct, req.Td, req.From, req.To)
		if err != nil {
			klog.Warnf("ResolveCycleRange err: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidTimeRange, "invalid time range", nil))
			return
		}

		dao, err := models.NewDBModel(c.GetString(constants.InstanceIdKey))
		if err != nil {
			klog.Errorf("NewDBModel error %s", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "internal error", nil))
			return
		}
		result, err := dao.QueryCategoryStat(from, to, req.Lg)
		if err != nil {
			klog.Errorf("统计收支失败: %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "statistic failed", nil))
			return
		}
		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, result))
	}
}

// ExpenseRank 支出分类 TOP 排行
func ExpenseRank() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		var req QueryArgs
		if err := c.BindAndValidate(&req); err != nil {
			klog.Error("BindAndValidate ", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeQueryError, "invalid query parameter", nil))
			return
		}
		from, to, _, _, err := ResolveCycleRange(req.Ct, req.Td, req.From, req.To)
		if err != nil {
			klog.Warnf("ResolveCycleRange err: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidTimeRange, "invalid time range", nil))
			return
		}

		dao, err := models.NewDBModel(c.GetString(constants.InstanceIdKey))
		if err != nil {
			klog.Errorf("NewDBModel error %s", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "internal error", nil))
			return
		}
		result, err := dao.QueryExpenseRank(from, to, 10, req.Dt)
		if err != nil {
			klog.Errorf("统计收支失败: %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "statistic failed", nil))
			return
		}
		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, result))
	}
}
