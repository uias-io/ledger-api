package category

import (
	"context"
	"errors"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/service/instance"
	"ledger/src/zlog"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

type ReqUpdateCategory struct {
	Name *string `json:"name,omitempty"`
}

func UpdateCategory() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := zlog.FromCtx(ctx)
		klog.Info("UpdateCategory")
		userId := c.GetString("userId")

		kid := strToInt64(c.Param("id"))
		if kid <= 0 {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidPathParam, "invalid category id", ""))
			return
		}

		var req ReqUpdateCategory
		if err := c.BindJSON(&req); err != nil {
			klog.Errorf("bind json failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to parse request data. Check JSON format.", ""))
			return
		}

		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", ""))
			return
		}

		// 分类必须存在且属于当前实例
		if _, err := models.FindCategoryById(instanceId, kid); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				klog.Errorf("category not found %v", err)
				c.JSON(http.StatusNotFound,
					answer.ResBody(answer.EcodeCategoryNotFound,
						"category not found.", nil))
				return
			}
			klog.Errorf("find category by id failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError,
					"Internal server error.", nil))
			return
		}

		fields := make(map[string]interface{})
		if req.Name != nil {
			fields["name"] = *req.Name
		}
		if len(fields) == 0 {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeNoFieldsToUpdate, "no fields to update", ""))
			return
		}

		if err := models.UpdateCategoryFields(instanceId, kid, fields); err != nil {
			klog.Errorf("update category failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "update category failed.", nil))
			return
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, nil))
	}
}
