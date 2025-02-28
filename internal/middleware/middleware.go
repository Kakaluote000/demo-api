package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redsync/redsync/v4"
	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/models"
	"github.com/kakaluote000/demo-api/pkg"

	"gorm.io/gorm"
)

var log = pkg.Log

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Log the request details
		start := time.Now()

		c.Next()

		end := time.Now()
		latency := end.Sub(start)

		ClientIP := c.ClientIP()
		method := c.Request.Method
		path := c.Request.URL.Path
		statusCode := c.Writer.Status()

		// 打印日志信息
		log.Printf("LoggerMiddleware 请求日志信息 %s %s %s %d %s", ClientIP, method, path, statusCode, latency)
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if token := c.GetHeader("Authorization"); token != "valid_token" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		c.Next()
		defer func() {
			// Log the response details
			log.Printf("AuthMiddleware 响应日志信息 %s %s %s %d", c.ClientIP(), c.Request.Method, c.Request.URL.Path, c.Writer.Status())
		}()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 设置允许跨域的源，这里使用 * 表示允许所有源
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		// 设置允许的请求方法
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		// 设置允许的请求头
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		// 设置允许携带凭证（如 cookies）
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		// 处理预检请求 (OPTIONS 请求)
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// 事务中间件
func TransactionMiddleware(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tx := db.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
			c.Abort()
			return
		}

		// 将事务存储在上下文中
		c.Set("tx", tx)

		c.Next()

		if c.Writer.Status() >= http.StatusBadRequest {
			tx.Rollback()
		} else {
			if err := tx.Commit().Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
				c.Abort()
				return
			}
		}
	}
}

// 分布式锁中间件
func DistributedLockMiddleware(app *app.App, lockNamePrefix string, expiry time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var userCurrency models.UserCurrency
		if err := c.ShouldBindJSON(&userCurrency); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// 将解析后的数据存储在上下文中
		c.Set("userCurrency", userCurrency)

		// 生成动态锁键
		lockName := fmt.Sprintf("%s:%d", lockNamePrefix, userCurrency.UserID)
		mutex, acquired := acquireLock(app.RS, lockName, expiry)
		if !acquired {
			c.JSON(http.StatusConflict, gin.H{"error": "Resource is locked"})
			c.Abort()
			return
		}
		defer releaseLock(mutex)

		c.Next()
	}
}

func acquireLock(rs *redsync.Redsync, name string, expiry time.Duration) (*redsync.Mutex, bool) {
	mutex := rs.NewMutex(name, redsync.WithExpiry(expiry))
	if err := mutex.Lock(); err != nil {
		log.Errorf("Failed to acquire lock: %v", err)
		return nil, false
	}
	return mutex, true
}

func releaseLock(mutex *redsync.Mutex) {
	if _, err := mutex.Unlock(); err != nil {
		log.Errorf("Failed to release lock: %v", err)
	}
}
