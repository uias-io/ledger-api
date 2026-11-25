package transaction

import (
	"context"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/pkg/common"
	"ledger/src/service/instance"
	"ledger/src/zlog"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/shopspring/decimal"
)

// DetailQuery 明细查询参数
type DetailQuery struct {
	TransactionId *int64  `query:"tid,omitempty"`     // 按账单过滤
	Keyword       *string `query:"keyword,omitempty"` // 名称/备注模糊搜索
}

// DetailResp 明细返回项
type DetailResp struct {
	Kid           int64           `json:"id"`
	TransactionId int64           `json:"tid"`
	Name          string          `json:"name"`
	Quantity      decimal.Decimal `json:"quantity"`
	Unit          string          `json:"unit"`
	Price         decimal.Decimal `json:"price"`
	Total         decimal.Decimal `json:"total"`
	Remark        string          `json:"remark"`
	OccAt         int64           `json:"occ_at"`
}

// SelectDetail 仅查询明细列表
func SelectDetail() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := zlog.FromCtx(ctx)
		userId := c.GetString("userId")

		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", ""))
			return
		}

		limit, offset, err := common.GetPagingQuery(c)
		if err != nil {
			klog.Error(err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidPagingParam, "invalid paging parameter", ""))
			return
		}

		var q DetailQuery
		if err := c.BindQuery(&q); err != nil {
			klog.Errorf("bind query failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeQueryError, "invalid query parameter", ""))
			return
		}

		// 空白 keyword 视为不传
		if q.Keyword != nil && strings.TrimSpace(*q.Keyword) == "" {
			q.Keyword = nil
		}

		f := models.DetailFilter{
			TransactionId: q.TransactionId,
			Keyword:       q.Keyword,
			Limit:         limit,
			Offset:        offset,
		}

		count := int64(0)
		result, err := models.FindDetailByFilter(instanceId, f, &count)
		if err != nil {
			klog.Error("in db select detail error: ", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Internal server error.", ""))
			return
		}

		items := make([]*DetailResp, 0, len(result))
		for _, v := range result {
			items = append(items, &DetailResp{
				Kid:           v.Kid,
				TransactionId: v.TransactionId,
				Name:          v.Name,
				Quantity:      v.Quantity,
				Unit:          v.Unit,
				Price:         v.Price,
				Total:         v.Total,
				Remark:        v.Remark,
				OccAt:         v.OccAt,
			})
		}

		payload := map[string]any{
			"items":     items,
			"page_info": answer.SetPageInfo(limit, offset, count),
		}
		klog.Info("select details successfully.")
		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, "", payload))
	}
}
