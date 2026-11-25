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

// ReqCategorySort holds the nested category sort payload.
type ReqCategorySort struct {
	Ids []int64 `json:"ids"` // Complete list of category IDs in the desired order.
}

// ReqSort is the request body for sorting main (parent) categories.
type ReqSort struct {
	Direction int8            `json:"direction"` // Required: 1 = income, 2 = expense.
	Category  ReqCategorySort `json:"category"`  // Nested category sort data.
}

// ReqSortSub is the request body for sorting sub (child) categories.
type ReqSortSub struct {
	Category ReqCategorySort `json:"category"`
}

// sameSet reports whether two ID slices represent exactly the same set.
func sameSet(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[int64]struct{}, len(a))
	for _, v := range a {
		m[v] = struct{}{}
	}
	if len(m) != len(a) {
		return false // a itself contains duplicates
	}
	for _, v := range b {
		if _, ok := m[v]; !ok {
			return false
		}
	}
	return true
}

// hasDuplicate reports whether the given ID slice contains duplicate values.
func hasDuplicate(ids []int64) bool {
	seen := make(map[int64]struct{}, len(ids))
	for _, v := range ids {
		if _, ok := seen[v]; ok {
			return true
		}
		seen[v] = struct{}{}
	}
	return false
}

// SortCategoryMain sorts main (parent) categories.
// PUT /v1/category/sort/main
func SortCategoryMain() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := zlog.FromCtx(ctx)
		userId := c.GetString("userId")

		var req ReqSort
		if err := c.BindJSON(&req); err != nil {
			klog.Errorf("bind json failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to parse request data. Check JSON format.", ""))
			return
		}

		if req.Direction != 1 && req.Direction != 2 {
			klog.Warnf("invalid direction: %d", req.Direction)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeCategoryInvalidDirection,
					"direction must be 1 or 2", ""))
			return
		}

		if len(req.Category.Ids) == 0 {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidSortRequest,
					"category.ids must not be empty", ""))
			return
		}

		if hasDuplicate(req.Category.Ids) {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidSortRequest,
					"category.ids contains duplicate ids", ""))
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

		// Strict completeness check: the submitted set must equal all parent
		// categories of this instance under the given direction.
		allParentIds, err := models.FindParentIdsByDirection(instanceId, req.Direction)
		if err != nil {
			klog.Errorf("find parent ids failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Internal server error.", nil))
			return
		}
		if !sameSet(allParentIds, req.Category.Ids) {
			klog.Warnf("category.ids incomplete for direction=%d", req.Direction)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeSortCategoryIncomplete,
					"category.ids must contain all parent categories in this scope", ""))
			return
		}

		if err := models.SortCategoryParent(instanceId, req.Direction, req.Category.Ids); err != nil {
			klog.Errorf("sort parent category failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "sort parent category failed.", nil))
			return
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, nil))
	}
}

// SortCategorySub sorts sub (child) categories under a given parent category.
// PUT /v1/category/:categoryId/sort/sub
func SortCategorySub() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := zlog.FromCtx(ctx)
		userId := c.GetString("userId")

		parentId := strToInt64(c.Param("categoryId"))
		if parentId <= 0 {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidPathParam,
					"invalid categoryId in path", ""))
			return
		}

		var req ReqSortSub
		if err := c.BindJSON(&req); err != nil {
			klog.Errorf("bind json failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to parse request data. Check JSON format.", ""))
			return
		}

		if len(req.Category.Ids) == 0 {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidSortRequest,
					"category.ids must not be empty", ""))
			return
		}

		if hasDuplicate(req.Category.Ids) {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidSortRequest,
					"category.ids contains duplicate ids", ""))
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

		// The parent category must exist, belong to the current instance,
		// and be a main (layer = 1) category.
		parent, err := models.FindCategoryById(instanceId, parentId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				klog.Errorf("parent category not found %v", err)
				c.JSON(http.StatusNotFound,
					answer.ResBody(answer.EcodeCategoryNotFound,
						"parent category not found.", nil))
				return
			}
			klog.Errorf("find parent category failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Internal server error.", nil))
			return
		}
		if parent.Layer != 1 {
			klog.Warnf("parent category is not layer1, kid=%d", parent.Kid)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeSortCategoryMismatch,
					"parent category is not a main category", ""))
			return
		}

		// Strict completeness check: the submitted set must equal all child
		// categories of this parent category.
		childIds, err := models.FindChildrenIdsByParent(parentId)
		if err != nil {
			klog.Errorf("find children ids failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Internal server error.", nil))
			return
		}
		if !sameSet(childIds, req.Category.Ids) {
			klog.Warnf("category.ids incomplete for parentId=%d", parentId)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeSortCategoryIncomplete,
					"category.ids must contain all child categories of this parent", ""))
			return
		}

		if err := models.SortCategoryChildren(instanceId, parentId, req.Category.Ids); err != nil {
			klog.Errorf("sort sub category failed %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "sort sub category failed.", nil))
			return
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, nil))
	}
}
