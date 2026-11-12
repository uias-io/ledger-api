package route

import (
	"context"
	"ledger/src/service/category"
	"ledger/src/service/statistic"
	"ledger/src/service/tag"
	"ledger/src/service/transaction"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
)

func Middleware(h *server.Hertz) {
	h.Use(RequestId())
	h.Use(RequestRecorder())
}

func helloWorld() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, utils.H{"message": "Hello World."})
	}
}

func AppRoute(h *server.Hertz) {
	h.HEAD("")
	h.GET("/", helloWorld())

	h.POST("/v1/category/main", apc("ledger:category:create"), category.CreateCategoryMain())           // 添加主分类 √
	h.POST("/v1/category/:categoryId/sub", apc("ledger:category:create"), category.CreateCategorySub()) // 添加子分类 √
	h.DELETE("/v1/category/:kid", apc("ledger:category:delete"), category.DeleteCategory())             // 删除分类
	h.GET("/v1/category", apc("ledger:category:list"), category.SelectCategory())                       // 查询分类 √
	h.PATCH("/v1/category/:id", apc("ledger:category:update"), category.UpdateCategory())               // 修改分类

	h.POST("/v1/transactions", apc("ledger:bill:create"), transaction.CreateTransaction())       // 添加记账流水 √
	h.GET("/v1/transactions", apc("ledger:bill:list"), transaction.SelectTransaction())          // 查询记账流水 √
	h.PATCH("/v1/transactions/:id", apc("ledger:bill:update"), transaction.UpdateTransaction())  // 修改
	h.DELETE("/v1/transactions/:id", apc("ledger:bill:delete"), transaction.DeleteTransaction()) // 删除 √
	h.GET("/v1/transactions/:id", apc("ledger:bill:get"), transaction.BillDetails())             // 账单详情ledger:bill:get
	h.GET("/v1/bill/:id", apc("ledger:bill:get"), transaction.BillDetails())                     // 账单详情ledger:bill:get

	h.GET("/v1/tags/hot", apc("ledger:tag:list"), tag.HotTags())       // √ 高频推荐标签
	h.GET("/v1/tags/search", apc("ledger:tag:list"), tag.SearchTags()) // √ 模糊搜索历史标签
	h.GET("/v1/bill/:id/tags", apc("ledger:bill:get"), tag.BillTags()) // √ 查询某条账单全部标签

	h.GET("/v1/bill/statistic", apc("ledger:overview:list"), statistic.StatOve())          // √ 统计收支,年，月，自定义
	h.GET("/v1/stat/cycle-summary", apc("ledger:overview:list"), statistic.CycleSummary()) // √ 月度汇总
	h.GET("/v1/stat/chart/day", apc("ledger:overview:list"), statistic.DayTrend())         // √ 日收支趋势
	h.GET("/v1/stat/chart/month", apc("ledger:overview:list"), statistic.MonthTrend())     // √ 月收支趋势
	h.GET("/v1/stat/chart/year", apc("ledger:overview:list"), statistic.YearTrend())       // √ 年收支趋势
	h.GET("/v1/stat/category", apc("ledger:overview:list"), statistic.CategoryPie())       // √ 收支分类构成
	h.GET("/v1/stat/expense-rank", apc("ledger:overview:list"), statistic.ExpenseRank())   // √ 本月支出分类TOP排行
}
