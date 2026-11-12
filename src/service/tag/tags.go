package tag

import (
	"context"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/service/instance"
	"ledger/src/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/cloudwego/hertz/pkg/app"
)

// HotTags 获取高频推荐标签
// GET /v1/tags/hot?limit=10
func HotTags() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")

		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", nil))
			return
		}

		// limit 默认 10，范围 [1, 50]
		limit := 10
		if v := c.Query("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				c.JSON(http.StatusBadRequest,
					answer.ResBody(answer.EcodeInvalidRequestError, "invalid limit", nil))
				return
			}
			if n > 50 {
				n = 50
			}
			limit = n
		}

		list, err := models.ListHotTags(instanceId, limit)
		if err != nil {
			klog.Errorf("list hot tags failed: %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Failed to get hot tags.", nil))
			return
		}
		if list == nil {
			list = make([]models.TagStat, 0)
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, "", map[string]any{
			"list": list,
		}))
	}
}

// SearchTags 模糊搜索历史标签
// GET /v1/tags/search?keyword=xx&limit=10
func SearchTags() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")

		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", nil))
			return
		}

		keyword := strings.TrimSpace(c.Query("keyword"))
		if keyword == "" {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "keyword must not be empty", nil))
			return
		}

		limit := 10
		if v := c.Query("limit"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				c.JSON(http.StatusBadRequest,
					answer.ResBody(answer.EcodeInvalidRequestError, "invalid limit", nil))
				return
			}
			if n > 50 {
				n = 50
			}
			limit = n
		}

		list, err := models.SearchTags(instanceId, keyword, limit)
		if err != nil {
			klog.Errorf("search tags failed: %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Failed to search tags.", nil))
			return
		}
		if list == nil {
			list = make([]models.TagStat, 0)
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, "", map[string]any{
			"list": list,
		}))
	}
}

// BillTags 查询某条账单全部标签
// GET /v1/bill/:id/tags
func BillTags() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")

		instanceId, err := instance.GetInstanceId(userId)
		if err != nil {
			klog.Errorf("get instance id failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to get your account info. Try again later.", nil))
			return
		}

		type Query struct {
			BillId int64 `path:"id"`
		}
		var q Query
		if err := c.BindPath(&q); err != nil {
			klog.Errorf("bind path failed: %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "invalid query parameter", nil))
			return
		}
		if q.BillId <= 0 {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError, "invalid bill id", nil))
			return
		}

		names, err := models.GetBillTagNames(instanceId, q.BillId)
		if err != nil {
			klog.Errorf("get bill tags failed: %v", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError, "Failed to get bill tags.", nil))
			return
		}
		if names == nil {
			names = make([]string, 0)
		}

		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, "", map[string]any{
			"tags": names,
		}))
	}
}
