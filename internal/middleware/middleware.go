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
	"github.com/kakaluote000/demo-api/pkg/auth"
	"github.com/kakaluote000/demo-api/pkg/metrics"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

var log = pkg.Log

func LoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path

		// 记录请求计数
		metrics.RequestCounter.WithLabelValues(method, path, fmt.Sprintf("%d", statusCode)).Inc()

		// 记录请求持续时间
		metrics.RequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())

		// 原有的日志记录保持不变
		log.Printf("LoggerMiddleware 请求日志信息 %s %s %s %d %s",
			c.ClientIP(), method, path, statusCode, duration)
	}
}

// 更新认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No token provided"})
			c.Abort()
			return
		}

		// 移除 Bearer 前缀
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		claims, err := auth.ParseToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// 将用户信息存储在上下文中
		c.Set("userID", claims.UserID)
		c.Next()
	}
}

// 添加限流中间件
func RateLimitMiddleware() gin.HandlerFunc {
	limiter := rate.NewLimiter(rate.Every(time.Second), 100) // 每秒100个请求
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
			c.Abort()
			return
		}
		c.Next()
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
