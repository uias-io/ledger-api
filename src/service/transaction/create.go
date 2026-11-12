package transaction

import (
	"context"
	"fmt"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/pkg/common"
	"ledger/src/service/instance"
	"ledger/src/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/shopspring/decimal"
)

// Detail 账目明细
type Detail struct {
	Name     string          `json:"name"`     // 物品名称（如“牛奶”）
	Quantity decimal.Decimal `json:"quantity"` // 数量（如：2件商品、2次服务）
	Unit     string          `json:"unit"`     // 单位
	Price    decimal.Decimal `json:"price"`    // 单价
	Total    decimal.Decimal `json:"total"`    // 总价（数目 × 单价，与原 amount 绝对值匹配）
}

// ReqTransaction 请求数据
type ReqTransaction struct {
	Cid     int64           `json:"cid"`      // 类型ID
	OccTime string          `json:"occ_time"` // 发生的时间ISO 8601, 20220909T18:47:66+0800
	Amount  decimal.Decimal `json:"amount"`   // 金额 0.0000
	Remark  string          `json:"remark"`   // 备注
	Total   int8            `json:"total"`    // 是否计入本月收支，1：计入；0：不计入
	Detail  []*Detail       `json:"detail"`   // 明细
	Tags    []string        `json:"tags"`     // 标签名称数组

}

// isInvalidOccAt 判断 DAO 返回的错误是否为 occ_at 非法
func isInvalidOccAt(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid occ_at")
}

// parseOccTime 解析 ISO 8601 时间字符串，返回规范化字符串和毫秒时间戳。
// occ_time / occ_at 的唯一派生入口，解析失败显式报错。
func parseOccTime(timeStr string) (string, int64, error) {
	layout := "2006-01-02T15:04:05-0700"
	t, err := time.Parse(layout, timeStr)
	if err != nil {
		return "", 0, fmt.Errorf("invalid occ_time %q: %w", timeStr, err)
	}
	return t.Format(layout), t.UnixMilli(), nil
}

// validateDetailTotal 校验明细总价之和不超过账单金额绝对值
func validateDetailTotal(amount decimal.Decimal, details []*Detail) error {
	if len(details) == 0 {
		return nil
	}
	sum := decimal.Zero
	for _, d := range details {
		sum = sum.Add(d.Total)
	}
	if sum.GreaterThan(amount.Abs()) {
		return fmt.Errorf("detail total %s exceeds amount %s", sum.String(), amount.Abs().String())
	}
	return nil
}

// toOrmDetails 将请求明细转换为 ORM 明细
func toOrmDetails(details []*Detail) []*models.OrmDetail {
	if len(details) == 0 {
		return nil
	}
	ormDetails := make([]*models.OrmDetail, 0, len(details))
	for _, val := range details {
		ormDetails = append(ormDetails, &models.OrmDetail{
			Name:     val.Name,
			Quantity: val.Quantity,
			Price:    val.Price,
			Unit:     val.Unit,
			Total:    val.Total,
		})
	}
	return ormDetails
}

func CreateTransaction() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")
		var ReqData ReqTransaction
		if err := c.BindJSON(&ReqData); err != nil {
			klog.Errorf("bind json failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to parse request data. Check JSON format.", ""))
			return
		}

		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed  %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", ""))
			return
		}

		occTime, occAt, err := parseOccTime(ReqData.OccTime)
		if err != nil {
			klog.Errorf("parse occ_time failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "invalid occ_time", ""))
			return
		}

		// 明细总价不得超过金额绝对值
		if err := validateDetailTotal(ReqData.Amount, ReqData.Detail); err != nil {
			klog.Warnf("validate detail total failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"detail total must not exceed amount", ""))
			return
		}

		now := common.CreateTimestamp()
		dbData := &models.OrmTransaction{
			InstanceId: instanceId,
			CategoryId: ReqData.Cid,
			Amount:     ReqData.Amount,
			OccTime:    occTime,
			OccAt:      occAt,
			Remark:     ReqData.Remark,
			UpdateTime: now,
			CreateTime: now,
		}
		dao, err := models.NewDBModel(instanceId)
		if err != nil {
			klog.Errorf("NewDBModel error %s", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, err.Error(), nil))
			return
		}

		if err := dao.InstallTransaction(dbData, toOrmDetails(ReqData.Detail), ReqData.Tags); err != nil {
			klog.Errorf("insert transaction failed %v", err)
			if isInvalidOccAt(err) {
				c.JSON(http.StatusBadRequest,
					answer.ResBody(answer.EcodeInvalidRequestError, "invalid occ_time", ""))
				return
			}
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Failed to create transaction. Try again later.", ""))
			return
		}

		c.JSON(http.StatusCreated, answer.ResBody(answer.EcodeOK, nil, nil))
	}
}
