package transaction

import (
	"context"
	"errors"
	"fmt"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/service/instance"
	"ledger/src/slog"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/shopspring/decimal"
)

type ReqDetailItem struct {
	Name     *string          `json:"name,omitempty"`     // 物品名称
	Quantity *decimal.Decimal `json:"quantity,omitempty"` // 数量
	Unit     *string          `json:"unit,omitempty"`     // 单位
	Price    *decimal.Decimal `json:"price,omitempty"`    // 单价
	Total    *decimal.Decimal `json:"total,omitempty"`    // 总价
}

type ReqUpdateTransaction struct {
	Cid     *int64           `json:"cid,omitempty"`      // 类型ID
	OccTime *string          `json:"occ_time,omitempty"` // 发生的时间ISO 8601
	Amount  *decimal.Decimal `json:"amount,omitempty"`   // 金额 0.0000
	Remark  *string          `json:"remark,omitempty"`   // 备注
	Total   *int8            `json:"total,omitempty"`    // 是否计入本月收支，1：计入；0：不计入
	Detail  *[]ReqDetailItem `json:"detail,omitempty"`   // 账目明细
	Tags    []string         `json:"tags"`               // 标签名称数组
}

// BuildTransUpdateMap 只提取主交易表字段；occ_time/occ_at 成对写入
func BuildTransUpdateMap(req *ReqUpdateTransaction) (map[string]any, error) {
	m := make(map[string]any)
	if req.Cid != nil {
		m["category_id"] = *req.Cid
	}
	if req.OccTime != nil {
		occTime, occAt, err := parseOccTime(*req.OccTime)
		if err != nil {
			return nil, err
		}
		m["occ_time"] = occTime
		m["occ_at"] = occAt
	}
	if req.Amount != nil {
		m["amount"] = *req.Amount
	}
	if req.Remark != nil {
		m["remark"] = *req.Remark
	}
	if req.Total != nil {
		m["total"] = *req.Total
	}
	return m, nil
}

// ConvertToOrmDetail 请求明细转 OrmDetail 数组
func ConvertToOrmDetail(tid int64, items []ReqDetailItem) ([]*models.OrmDetail, error) {
	var list []*models.OrmDetail
	for _, v := range items {
		row := models.OrmDetail{TransactionId: tid}
		if v.Name != nil {
			row.Name = *v.Name
		}
		if v.Quantity != nil {
			row.Quantity = *v.Quantity
		}
		if v.Unit != nil {
			row.Unit = *v.Unit
		}
		if v.Price != nil {
			row.Price = *v.Price
		}
		if v.Total != nil {
			row.Total = *v.Total
		}
		list = append(list, &row)
	}
	// 空数组表示"清空明细"，返回非 nil 空切片以区别于 nil
	if list == nil {
		list = make([]*models.OrmDetail, 0)
	}
	return list, nil
}

// validateUpdateDetailTotal 更新时校验明细总价不超过金额绝对值
// amount 取本次传入值；未传则查库取当前值
func validateUpdateDetailTotal(instanceId string, tid int64, reqAmount *decimal.Decimal, detailList []*models.OrmDetail) error {
	if detailList == nil {
		return nil
	}
	if len(detailList) == 0 {
		return nil
	}

	amount := reqAmount
	if amount == nil {
		billList, err := models.GetBillById(instanceId, tid)
		if err != nil {
			return fmt.Errorf("query transaction for amount failed: %w", err)
		}
		if len(billList) == 0 {
			return models.ErrTransactionNotFound
		}
		a := billList[0].Amount
		amount = &a
	}

	sum := decimal.Zero
	for _, d := range detailList {
		sum = sum.Add(d.Total)
	}
	if sum.GreaterThan(amount.Abs()) {
		return fmt.Errorf("detail total %s exceeds amount %s", sum.String(), amount.Abs().String())
	}
	return nil
}

func UpdateTransaction() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)

		var req ReqUpdateTransaction
		if err := c.BindJSON(&req); err != nil {
			klog.Errorf("bind json failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to parse request data. Check JSON format.", nil))
			return
		}

		tidStr := c.Param("id")
		tid, err := strconv.ParseInt(tidStr, 10, 64)
		if err != nil {
			klog.Error(err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "invalid transaction id", nil))
			return
		}

		userId := c.GetString("userId")
		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", nil))
			return
		}

		transMap, err := BuildTransUpdateMap(&req)
		if err != nil {
			klog.Error("build trans update map error", "err", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "invalid occ_time", nil))
			return
		}

		var detailList []*models.OrmDetail
		if req.Detail != nil {
			detailList, err = ConvertToOrmDetail(tid, *req.Detail)
			if err != nil {
				klog.Error("convert detail error", "err", err)
				c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, "parse detail failed", nil))
				return
			}
		}

		if len(transMap) == 0 && detailList == nil && req.Tags == nil {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "no fields to update", nil))
			return
		}

		// 明细总价校验：本次操作了明细才校验
		if err := validateUpdateDetailTotal(instanceId, tid, req.Amount, detailList); err != nil {
			klog.Warnf("validate detail total failed: %v", err)
			if errors.Is(err, models.ErrTransactionNotFound) {
				c.JSON(http.StatusNotFound,
					answer.ResBody(answer.EcodeInvalidRequestError, "transaction not found", nil))
				return
			}
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"detail total must not exceed amount", nil))
			return
		}

		dao, err := models.NewDBModel(instanceId)
		if err != nil {
			klog.Errorf("NewDBModel error %s", err)
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, err.Error(), nil))
			return
		}

		if err := dao.UpdateTransaction(tid, transMap, detailList, req.Tags); err != nil {
			klog.Error("dao UpdateTransaction failed", "err", err)
			if errors.Is(err, models.ErrTransactionNotFound) {
				c.JSON(http.StatusNotFound,
					answer.ResBody(answer.EcodeInvalidRequestError, "transaction not found", nil))
				return
			}
			if isInvalidOccAt(err) {
				c.JSON(http.StatusBadRequest,
					answer.ResBody(answer.EcodeInvalidRequestError, "invalid occ_time", nil))
				return
			}
			c.JSON(http.StatusInternalServerError, answer.ResBody(answer.EcodeError, "update failed", nil))
			return
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, "success", nil))
	}
}
