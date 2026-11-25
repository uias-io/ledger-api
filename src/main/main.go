package main

import (
	"context"
	"ledger/src/config"
	"ledger/src/database/mysql"
	"ledger/src/database/sql"
	"ledger/src/models"
	"ledger/src/pkg/migrator"
	"ledger/src/route"
	"ledger/src/zlog"
	"log/slog"
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"gorm.io/gorm"
)

func init() {
	config.Initializer()
}

// AutoMigrator performs automatic database migrations using embedded SQL scripts
func AutoMigrator(db *gorm.DB) {
	// Initialize migrator with embedded scripts
	m := migrator.NewDefault(db,
		&migrator.Config{
			Logger:         migrator.NewSlogAdapter(slog.Default()),
			TableName:      "ledger_migrations",
			ScriptProvider: migrator.NewEmbeddedScriptProvider("script", sql.SQLScript),
		})

	// Execute migration workflow
	if err := migrator.RunWithMigrator(m); err != nil {
		panic("Database migration failed: " + err.Error())
	}
}

// main is the entry point of the ledger application
func main() {
	ctx := context.Background()
	cfg := config.InitConfig(ctx)
	app := cfg.Ledger.App

	zlog.InitLogger(&cfg.Ledger.Log)
	klog := zlog.FromCtx(zlog.NewCtxWithReqID(ctx, "<SYSTEM>"))

	// Initialize database connection
	db := mysql.InitDB(cfg.Ledger)
	models.SetConnectPool(db)

	// Execute critical database migrations
	AutoMigrator(db)

	klog.Info("start server...")
	svc := server.Default(
		server.WithHostPorts(app.Bind),
		server.WithExitWaitTime(0*time.Second))

	route.Middleware(svc)
	route.AppRoute(svc)
	svc.Spin()
}
