package transaction

import (
	"context"
	"errors"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/service/instance"
	"ledger/src/slog"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
)

// TODO 待移除：ensureTransactionOwned

func DeleteTransaction() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")

		// 查询用户的实例ID
		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed  %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", ""))
			return
		}

		type Query struct {
			TransactionId int64 `path:"id"`
		}
		var q Query
		if err := c.BindPath(&q); err != nil {
			klog.Errorf("bind path failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidPathParam, "invalid transaction id.", nil))
			return
		}

		// server 层校验：交易必须存在且属于当前实例
		billList, err := models.GetBillById(instanceId, q.TransactionId)
		if err != nil {
			klog.Errorf("get bill by id failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError,
					"Internal server error.", nil))
			return
		}
		if len(billList) == 0 {
			klog.Warnf("transaction not found id=%d instance=%s", q.TransactionId, instanceId)
			c.JSON(http.StatusNotFound,
				answer.ResBody(answer.EcodeTransactionNotFound,
					"transaction not found.", nil))
			return
		}

		dao, err := models.NewDBModel(instanceId)
		if err != nil {
			klog.Errorf("NewDBModel error %s", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, err.Error(), nil))
			return
		}

		if err := dao.DeleteTransaction(q.TransactionId); err != nil {
			if errors.Is(err, models.ErrTransactionNotFound) {
				klog.Warnf("transaction not found id=%d instance=%s", q.TransactionId, instanceId)
				c.JSON(http.StatusNotFound,
					answer.ResBody(answer.EcodeTransactionNotFound,
						"transaction not found.", nil))
				return
			}

			klog.Errorf("delete transaction failed  %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError,
					"delete transaction failed.", nil))
			return
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, nil))
	}
}
