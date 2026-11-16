package category

import (
	"context"
	"errors"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/service/instance"
	"ledger/src/slog"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

func DeleteCategory() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		klog.Info("DeleteCategory")
		userId := c.GetString("userId")
		categoryId := c.Param("kid")

		kid := strToInt64(categoryId)
		if kid <= 0 {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidPathParam, "invalid categoryId", ""))
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

		categ, err := models.FindCategoryById(instanceId, kid)
		if err != nil {
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

		if categ.Layer == 1 {
			childIds, err := models.FindChildrenIdsByParent(categ.Kid)
			if err != nil {
				klog.Errorf("find children ids failed %v", err)
				c.JSON(http.StatusInternalServerError,
					answer.ResBody(answer.EcodeError, "Internal server error.", nil))
				return
			}
			if len(childIds) > 0 {
				klog.Warnf("category %d has children, cannot delete", categ.Kid)
				c.JSON(http.StatusBadRequest,
					answer.ResBody(answer.EcodeCategoryHasChildren,
						"category has children, cannot delete", ""))
				return
			}
		}

		if err := models.DeleteCategoryWithRel(instanceId, categ.Kid); err != nil {
			klog.Errorf("delete category failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError,
					"delete category failed.", nil))
			return
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, nil))
	}
}
