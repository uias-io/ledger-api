package statistic

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
)

func RecentSixMonthChartTrend() func(ctx context.Context, c *app.RequestContext) {
	return func(ctx context.Context, c *app.RequestContext) {
		//klog := zlog.FromCtx(ctx)
		//now := time.Now()
	}
}
