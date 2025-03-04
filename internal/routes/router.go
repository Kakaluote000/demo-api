package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/handlers"
	"github.com/kakaluote000/demo-api/internal/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func SetupRoutes(app *app.App) {
	router := app.Router
	
	// 全局中间件
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.CORSMiddleware())
	router.Use(middleware.RateLimitMiddleware())

	// 监控和健康检查路由
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", handlers.HealthCheckHandler())
	router.GET("/readiness", handlers.ReadinessCheckHandler(app))

	// 公开路由
	public := router.Group("/")
	{
		public.POST("/login", handlers.LoginHandler(app))
		public.POST("/register", handlers.RegisterHandler(app))
	}

	// 需要认证的路由
	authorized := router.Group("/")
	authorized.Use(middleware.AuthMiddleware())
	{
		authorized.GET("/userCurrency/:id", handlers.GetUserCurrencyHandler(app))
		authorized.POST("/userCurrency", handlers.AddUserCurrencyHandler(app))
		authorized.POST("/updateUserCurrency", handlers.UpdateUserCurrencyHandler(app))
		authorized.POST("/addCurrencyNum", 
			middleware.TransactionMiddleware(app.DB), 
			middleware.DistributedLockMiddleware(app, "add_currency_lock", 10*time.Second), 
			handlers.AddCurrencyNumHandler(app))
		authorized.POST("/subtractCurrencyNum", 
			middleware.TransactionMiddleware(app.DB), 
			middleware.DistributedLockMiddleware(app, "subtract_currency_lock", 10*time.Second), 
			handlers.SubtractCurrencyNumHandler(app))
	}

	// 监控相关路由
	monitoring := router.Group("/monitoring")
	{
		monitoring.POST("/webhook", handlers.AlertWebhookHandler(app))
	}
}
