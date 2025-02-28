package routes

import (
	"time"

	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/handlers"
	"github.com/kakaluote000/demo-api/internal/middleware"
)

func SetupRoutes(app *app.App) {
	router := app.Router
	router.Use(middleware.LoggerMiddleware())
	router.Use(middleware.AuthMiddleware())
	router.Use(middleware.CORSMiddleware())

	// Define a POST route for login
	router.POST("/login", handlers.LoginHandler(app))
	router.POST("/register", handlers.RegisterHandler(app))
	router.GET("/userCurrency/:id", handlers.GetUserCurrencyHandler(app))
	router.POST("/userCurrency", handlers.AddUserCurrencyHandler(app))
	router.POST("/updateUserCurrency", handlers.UpdateUserCurrencyHandler(app))

	// 应用事务中间件和分布式锁中间件
	router.POST("/addCurrencyNum", middleware.TransactionMiddleware(app.DB), middleware.DistributedLockMiddleware(app, "add_currency_lock", 10*time.Second), handlers.AddCurrencyNumHandler(app))
	router.POST("/subtractCurrencyNum", middleware.TransactionMiddleware(app.DB), middleware.DistributedLockMiddleware(app, "subtract_currency_lock", 10*time.Second), handlers.SubtractCurrencyNumHandler(app))
}
