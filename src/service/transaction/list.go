package transaction

import (
	"context"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/pkg/common"
	"ledger/src/service/instance"
	"ledger/src/slog"
	"net/http"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/shopspring/decimal"
)

// TransactionQuery 账单查询参数；时间桶语义 [From, To)，单位毫秒
type TransactionQuery struct {
	CategoryId *int64  `query:"cid,omitempty"`
	From       *int64  `query:"from,omitempty"`
	To         *int64  `query:"to,omitempty"`
	MinAmount  *string `query:"min_amount,omitempty"`
	MaxAmount  *string `query:"max_amount,omitempty"`
	Keyword    *string `query:"keyword,omitempty"`
	Direction  *int8   `query:"direction,omitempty"`
	Sort       *string `query:"sort,omitempty"`
	Order      *string `query:"order,omitempty"`
}

// buildTransactionFilter 解析并校验查询参数，返回错误消息（空串表示通过）
func buildTransactionFilter(q *TransactionQuery, limit, offset int) (models.TransactionFilter, string) {
	f := models.TransactionFilter{
		CategoryId: q.CategoryId,
		From:       q.From,
		To:         q.To,
		Keyword:    q.Keyword,
		Direction:  q.Direction,
		Limit:      limit,
		Offset:     offset,
		SortColumn: "occ_at",
		SortOrder:  "DESC",
	}

	// 时间桶
	if q.From != nil && *q.From < 0 {
		return f, "from must be >= 0"
	}
	if q.To != nil && *q.To < 0 {
		return f, "to must be >= 0"
	}
	if q.From != nil && q.To != nil && *q.From > *q.To {
		return f, "from must be <= to"
	}

	// 金额区间（正数语义，后端展开正负两段查询）
	if q.MinAmount != nil && *q.MinAmount != "" {
		d, err := decimal.NewFromString(*q.MinAmount)
		if err != nil {
			return f, "invalid min_amount"
		}
		if d.IsNegative() {
			return f, "min_amount must be >= 0"
		}
		f.MinAmount = &d
	}
	if q.MaxAmount != nil && *q.MaxAmount != "" {
		d, err := decimal.NewFromString(*q.MaxAmount)
		if err != nil {
			return f, "invalid max_amount"
		}
		if d.IsNegative() {
			return f, "max_amount must be >= 0"
		}
		f.MaxAmount = &d
	}
	if f.MinAmount != nil && f.MaxAmount != nil && f.MinAmount.GreaterThan(*f.MaxAmount) {
		return f, "min_amount must be <= max_amount"
	}

	// 收支方向白名单
	if q.Direction != nil && *q.Direction != 1 && *q.Direction != 2 {
		return f, "direction must be 1 or 2"
	}

	// keyword 空白视为不传
	if f.Keyword != nil && strings.TrimSpace(*f.Keyword) == "" {
		f.Keyword = nil
	}

	// 排序白名单
	if q.Sort != nil {
		switch *q.Sort {
		case "occ_at", "amount", "create_time":
			f.SortColumn = *q.Sort
		default:
			return f, "invalid sort_by"
		}
	}
	if q.Order != nil {
		switch strings.ToUpper(*q.Order) {
		case "ASC":
			f.SortOrder = "ASC"
		case "DESC":
			f.SortOrder = "DESC"
		default:
			return f, "invalid order_by"
		}
	}

	return f, ""
}

func SelectTransaction() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")

		// 1. 实例 ID
		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", ""))
			return
		}

		// 2. 分页（复用原有函数）
		limit, offset, err := common.GetPagingQuery(c)
		if err != nil {
			klog.Error(err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "invalid paging parameter", ""))
			return
		}

		// 3. 绑定搜索条件
		var q TransactionQuery
		if err := c.BindQuery(&q); err != nil {
			klog.Errorf("bind query failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "invalid query parameter", ""))
			return
		}

		// 4. 校验 + 组装 filter
		filter, errMsg := buildTransactionFilter(&q, limit, offset)
		if errMsg != "" {
			klog.Warnf("invalid query: %s", errMsg)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, errMsg, ""))
			return
		}

		// 5. 查询
		count := int64(0)
		result, err := models.FindTransactionByFilter(instanceId, filter, &count)
		if err != nil {
			klog.Error("in db select transaction error: ", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Internal server error.", ""))
			return
		}

		// 6. 组装响应（保持原有结构）
		type Category struct {
			Name string `json:"name,omitempty"`
			Id   int64  `json:"id,omitempty"`
		}
		type resp struct {
			Kid        int64           `json:"id"`
			CategoryId int64           `json:"category_id"`
			Category   Category        `json:"category"`
			Amount     decimal.Decimal `json:"amount"`
			OccTime    int64           `json:"occ_time"` // 后续可能删掉
			OccAt      int64           `json:"occ_at"`
			Remark     string          `json:"remark"`
			UpdateTime int64           `json:"update_time"`
			CreateTime int64           `json:"create_time"`
		}
		data := make([]*resp, len(result))
		for k, v := range result {
			data[k] = &resp{
				Kid:        v.Kid,
				CategoryId: v.CategoryId,
				Category:   Category{Id: v.CategoryId},
				Amount:     v.Amount,
				OccTime:    v.OccAt,
				OccAt:      v.OccAt,
				Remark:     v.Remark,
				UpdateTime: v.UpdateTime,
				CreateTime: v.CreateTime,
			}
		}

		pageInfo := answer.SetPageInfo(limit, offset, count)
		payload := map[string]interface{}{
			"items":     data,
			"page_info": pageInfo,
		}
		klog.Info("select transactions successfully.")
		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, "", payload))
	}
}

// DetailItem 账单明细项
type DetailItem struct {
	Name     string          `json:"name"`     // 物品名称
	Quantity decimal.Decimal `json:"quantity"` // 数目
	Price    decimal.Decimal `json:"price"`    // 单价
	Total    decimal.Decimal `json:"total"`    // 总价 = 数量 × 单价
	Unit     string          `json:"unit"`     // 单位
}

type BillDetailResp struct {
	Cid     int64           `json:"cid"`      // 对应 CategoryId
	OccTime string          `json:"occ_time"` // 时间字符串原样返回
	Amount  decimal.Decimal `json:"amount"`   // 主账单金额
	Remark  string          `json:"remark"`   // 备注
	Detail  []DetailItem    `json:"detail"`   // 明细数组
	Tags    []string        `json:"tags"`     // 标签
}

// BillDetails 账单详情
func BillDetails() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		klog.Debug("BillDetails")

		// 获取用户实例ID
		userId := c.GetString("userId")
		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed: %v", err)
			c.JSON(http.StatusBadRequest, answer.ResBody(answer.EcodeInvalidRequestError, "Failed to get your account info. Try again later.", nil))
			return
		}

		// 绑定路径参数 billId
		type Query struct {
			BillId int64 `path:"id"`
		}
		var q Query
		if err := c.BindPath(&q); err != nil {
			klog.Errorf("bind path failed: %v", err)
			c.JSON(http.StatusBadRequest, answer.ResBody(answer.EcodeInvalidRequestError, "invalid query parameter", nil))
			return
		}

		// 查询主账单
		billList, err := models.GetBillById(instanceId, q.BillId)
		if err != nil {
			klog.Errorf("get bill failed: %v", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, "get bill info error", nil))
			return
		}
		if len(billList) == 0 {
			c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, "bill not found", nil))
			return
		}
		bill := billList[0] // 单条账单

		// 查询账单明细
		detailList, err := models.GetDetail(q.BillId)
		if err != nil {
			klog.Errorf("get bill detail failed: %v", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, "get bill detail error", nil))
			return
		}

		// 组装明细结构
		detailItems := make([]DetailItem, 0, len(detailList))
		for _, d := range detailList {
			detailItems = append(detailItems, DetailItem{
				Name:     d.Name,
				Quantity: d.Quantity,
				Price:    d.Price,
				Total:    d.Total,
				Unit:     d.Unit,
			})
		}

		tags, err := models.GetBillTagNames(instanceId, q.BillId)
		if err != nil {
			klog.Errorf("get bill tags failed: %v", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, "get bill tags error", nil))
			return
		}
		if tags == nil {
			tags = make([]string, 0)
		}

		// 组装最终返回体
		respData := BillDetailResp{
			Cid:     bill.CategoryId,
			OccTime: bill.OccTime,
			Amount:  bill.Amount,
			Remark:  bill.Remark,
			Detail:  detailItems,
			Tags:    tags,
		}

		// 7. 返回成功响应
		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, "", respData))
	}
}
