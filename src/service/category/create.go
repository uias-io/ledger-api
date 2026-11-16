package category

import (
	"context"
	"errors"
	"fmt"
	"ledger/src/models"
	"ledger/src/pkg/answer"
	"ledger/src/pkg/common"
	"ledger/src/service/instance"
	"ledger/src/slog"
	"net/http"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"gorm.io/gorm"
)

type Category struct {
	Id        int64  `json:"id"`
	Name      string `json:"name"`
	Direction int8   `json:"direction"`
}

func CreateCategoryMain() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")

		var reqData Category
		if err := c.BindJSON(&reqData); err != nil {
			klog.Errorf("bind json failed %v", err)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidRequestError,
					"Failed to parse request data. Check JSON format.", ""))
			return
		}

		// direction 合法性校验
		if reqData.Direction != 1 && reqData.Direction != 2 {
			klog.Warnf("invalid direction: %d", reqData.Direction)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeCategoryInvalidDirection,
					"direction must be 1 or 2", ""))
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
		cate := &models.OrmCategory{
			InstanceId: instanceId,
			Name:       reqData.Name,
			Direction:  reqData.Direction,
			Layer:      1,
			CreateTime: common.CreateTimestamp(),
		}

		kid, err := models.CreateCategory(cate, nil)
		if err != nil {
			klog.Errorf("create category to db error: %s", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError,
					"create category to db error.", nil))
			return
		}

		reqData.Id = kid
		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, reqData))
	}
}

func strToInt64(s string) int64 {
	num, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		fmt.Println("转换错误:", err)
		return 0
	}
	return num
}

// CreateCategorySub 添加子分类
func CreateCategorySub() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		klog := slog.FromContext(c)
		userId := c.GetString("userId")

		parentId := strToInt64(c.Param("categoryId"))
		if parentId <= 0 {
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeInvalidPathParam,
					"invalid categoryId in path", ""))
			return
		}

		var reqData Category
		if err := c.BindJSON(&reqData); err != nil {
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

		categ, err := models.FindCategoryById(instanceId, parentId)
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

		// 不允许给子类再挂子类
		if categ.Layer != 1 {
			klog.Warnf("parent category is not layer1, kid=%d", categ.Kid)
			c.JSON(http.StatusBadRequest,
				answer.ResBody(answer.EcodeSortCategoryMismatch,
					"parent category is not a main category", ""))
			return
		}

		now := common.CreateTimestamp()
		cate := &models.OrmCategory{
			InstanceId: instanceId,
			Name:       reqData.Name,
			Direction:  categ.Direction, // 二级分类继承父类 Direction
			Layer:      2,
			CreateTime: now,
		}

		rela := &models.OrmCaterela{
			ParentId:   parentId,
			CreateTime: now,
		}

		kid, err := models.CreateCategory(cate, rela)
		if err != nil {
			klog.Errorf("create category to db error: %s", err)
			c.JSON(http.StatusInternalServerError,
				answer.ResBody(answer.EcodeError,
					"create category to db error.", nil))
			return
		}

		reqData.Id = kid
		c.JSON(http.StatusOK, answer.ResBody(answer.EcodeOK, nil, reqData))
	}
}
