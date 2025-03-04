package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kakaluote000/demo-api/cmd/app"
)

func HealthCheckHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "currency-management-system",
			"version": "1.0.0",
		})
	}
}

func ReadinessCheckHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查数据库连接
		sqlDB, err := app.DB.DB()
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": "Database connection error"})
			return
		}
		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": "Database ping failed"})
			return
		}

		// 检查Redis连接
		if err := app.Redis.Ping(app.Ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "message": "Redis connection error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
