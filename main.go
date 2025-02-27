package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var ctx = context.Background()
var rdb *redis.Client
var log *logrus.Logger
var rs *redsync.Redsync // 声明全局变量 rs

func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis 服务器地址
		Password: "",               // 密码
		DB:       0,                // 默认数据库
	})

	// 测试连接
	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}
	log.Printf("Redis connected: %s", pong)

	// 初始化 redsync
	pool := goredis.NewPool(rdb)
	rs = redsync.New(pool)
}

func initLogrus() {
	log = logrus.New()

	// 创建日志文件
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to log to file, using default stderr")
	}

	// 设置日志输出到文件
	log.SetOutput(file)

	// 设置日志格式
	log.SetFormatter(&logrus.JSONFormatter{})
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

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

		//打印日志信息
		log.Printf("LoggerMiddleware 请求日志信息%s %s %s %d %s", ClientIP, method, path, statusCode, latency)

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
			log.Printf("AuthMiddleware 响应日志信息%s %s %s %d", c.ClientIP(), c.Request.Method, c.Request.URL.Path, c.Writer.Status())
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
// 分布式锁中间件
func DistributedLockMiddleware(db *gorm.DB, lockNamePrefix string, expiry time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		var userCurrency UserCurrency
		if err := c.ShouldBindJSON(&userCurrency); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			c.Abort()
			return
		}

		// 将解析后的数据存储在上下文中
		c.Set("userCurrency", userCurrency)

		// 生成动态锁键
		lockName := fmt.Sprintf("%s:%d", lockNamePrefix, userCurrency.UserID)
		mutex, acquired := acquireLock(lockName, expiry)
		if !acquired {
			c.JSON(http.StatusConflict, gin.H{"error": "Resource is locked"})
			c.Abort()
			return
		}
		defer releaseLock(mutex)

		c.Next()
	}
}

func acquireLock(name string, expiry time.Duration) (*redsync.Mutex, bool) {
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

// User定义用户模型 对应的是user表
// User 定义用户模型，对应 user 表
type User struct {
	gorm.Model
	Username string `gorm:"column:username;not null;unique" json:"username"`
	Password string `gorm:"column:password;not null" json:"password"`
}

// UserCurrency定义用户模型 对应的是user_currency表
type UserCurrency struct {
	gorm.Model
	UserID      uint `gorm:"column:user_id;not null" json:"user_id"`
	CurrencyID  uint `gorm:"column:currency_id;not null" json:"currency_id"`
	CurrencyNum uint `gorm:"column:currency_num;not null" json:"currency_num"`
}

// CurrencyTransaction 定义货币交易模型，对应 currency_transaction 表
type CurrencyTransaction struct {
	gorm.Model
	UserID          uint      `gorm:"column:user_id;not null" json:"user_id"`
	CurrencyID      uint      `gorm:"column:currency_id;not null" json:"currency_id"`
	Amount          uint      `gorm:"column:amount;not null" json:"amount"`
	Type            string    `gorm:"column:type;not null" json:"type"` // "add" 或 "subtract"
	TransactionTime time.Time `gorm:"column:transaction_time;not null" json:"transaction_time"`
}

func RegisterHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var user User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 检查用户名是否已经存在
		var existingUser User
		if err := db.Where("username = ?", user.Username).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
		}
		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "User registered successfully"})
	}
}

func LoginHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var loginReq LoginRequest
		if err := c.ShouldBindJSON(&loginReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var user User
		result := db.Where("username = ? AND password = ?", loginReq.Username, loginReq.Password).First(&user)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Login successful"})
	}
}

func AddUserCurrencyHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var userCurrency UserCurrency
		if err := c.ShouldBindJSON(&userCurrency); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 检查用户是否存在
		var user User
		if err := db.Where("id = ?", userCurrency.UserID).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 创建用户货币记录
		if err := db.Create(&userCurrency).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add user currency"})
			return
		}

		// 清除缓存
		cacheKey := fmt.Sprintf("user_currency:%d", userCurrency.UserID)
		rdb.Del(ctx, cacheKey)

		c.JSON(http.StatusOK, gin.H{"message": "User currency added successfully"})
	}
}

func GetUserCurrencyHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cacheKey := fmt.Sprintf("user_currency:%s", id)

		// 尝试从缓存中获取数据
		cachedData, err := rdb.Get(ctx, cacheKey).Result()
		if err == nil {
			var userCurrency UserCurrency
			if err := json.Unmarshal([]byte(cachedData), &userCurrency); err == nil {
				c.JSON(http.StatusOK, userCurrency)
				return
			}
		}

		// 如果缓存中没有数据，则从数据库中获取
		var userCurrency UserCurrency
		if err := db.Where("user_id = ?", id).First(&userCurrency).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User currency not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 将数据存入缓存
		data, err := json.Marshal(userCurrency)
		if err == nil {
			rdb.Set(ctx, cacheKey, data, time.Hour) // 缓存有效期为1小时
		}

		c.JSON(http.StatusOK, userCurrency)
	}
}

func UpdateUserCurrencyHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var userCurrency UserCurrency
		if err := c.ShouldBindJSON(&userCurrency); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 检查用户是否存在
		var user User
		if err := db.Where("id = ?", userCurrency.UserID).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 更新用户货币记录
		if err := db.Model(&UserCurrency{}).Where("user_id = ? AND currency_id = ?", userCurrency.UserID, userCurrency.CurrencyID).Updates(UserCurrency{
			CurrencyNum: userCurrency.CurrencyNum,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user currency"})
			return
		}

		// 清除缓存
		cacheKey := fmt.Sprintf("user_currency:%d", userCurrency.UserID)
		rdb.Del(ctx, cacheKey)

		c.JSON(http.StatusOK, gin.H{"message": "User currency updated successfully"})
	}
}

func AddCurrencyNumHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文中获取解析后的userCurrency
		val, ok := c.Get("userCurrency")
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid userCurrency"})
			return
		}

		userCurrency := val.(UserCurrency)

		// 检查用户是否存在
		var user User
		if err := db.Where("id = ?", userCurrency.UserID).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 检查用户货币记录是否存在
		var existingUserCurrency UserCurrency
		if err := db.Where("user_id = ? AND currency_id = ?", userCurrency.UserID, userCurrency.CurrencyID).First(&existingUserCurrency).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User currency not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 增加货币数量
		newCurrencyNum := existingUserCurrency.CurrencyNum + userCurrency.CurrencyNum
		if err := db.Model(&UserCurrency{}).Where("user_id = ? AND currency_id = ?", userCurrency.UserID, userCurrency.CurrencyID).Updates(UserCurrency{
			CurrencyNum: newCurrencyNum,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user currency"})
			return
		}

		// 记录流水日志
		transaction := CurrencyTransaction{
			UserID:          userCurrency.UserID,
			CurrencyID:      userCurrency.CurrencyID,
			Amount:          userCurrency.CurrencyNum,
			Type:            "add",
			TransactionTime: time.Now(),
		}
		if err := db.Create(&transaction).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log transaction"})
			return
		}

		// 清除缓存
		cacheKey := fmt.Sprintf("user_currency:%d", userCurrency.UserID)
		rdb.Del(ctx, cacheKey)

		c.JSON(http.StatusOK, gin.H{"message": "User currency added successfully", "new_currency_num": newCurrencyNum})
	}
}

func SubtractCurrencyNumHandler(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从上下文中获取解析后的userCurrency
		val, ok := c.Get("userCurrency")
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid userCurrency"})
			return
		}

		userCurrency := val.(UserCurrency)

		// 检查用户是否存在
		var user User
		if err := db.Where("id = ?", userCurrency.UserID).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 检查用户货币记录是否存在
		var existingUserCurrency UserCurrency
		if err := db.Where("user_id = ? AND currency_id = ?", userCurrency.UserID, userCurrency.CurrencyID).First(&existingUserCurrency).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User currency not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 检查是否会导致负数
		if existingUserCurrency.CurrencyNum < userCurrency.CurrencyNum {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Insufficient currency"})
			return
		}

		// 扣减货币数量
		newCurrencyNum := existingUserCurrency.CurrencyNum - userCurrency.CurrencyNum
		if err := db.Model(&UserCurrency{}).Where("user_id = ? AND currency_id = ?", userCurrency.UserID, userCurrency.CurrencyID).Updates(UserCurrency{
			CurrencyNum: newCurrencyNum}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user currency"})
			return
		}

		// 记录流水日志
		transaction := CurrencyTransaction{
			UserID:          userCurrency.UserID,
			CurrencyID:      userCurrency.CurrencyID,
			Amount:          userCurrency.CurrencyNum,
			Type:            "subtract",
			TransactionTime: time.Now(),
		}
		if err := db.Create(&transaction).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to log transaction"})
			return
		}

		// 清除缓存
		cacheKey := fmt.Sprintf("user_currency:%d", userCurrency.UserID)
		rdb.Del(ctx, cacheKey)

		c.JSON(http.StatusOK, gin.H{"message": "User currency subtracted successfully", "new_currency_num": newCurrencyNum})
	}
}

func intMysql() *gorm.DB {
	dsn := "demo_db_user:demoAbc123@tcp(127.0.0.1:3306)/demo_db?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	return db
}

func main() {
	initLogrus()
	db := intMysql()
	initRedis()

	db.AutoMigrate(&User{})
	db.AutoMigrate(&UserCurrency{})
	db.AutoMigrate(&CurrencyTransaction{})

	// Create a new gin router
	router := gin.Default()
	router.Use(LoggerMiddleware())
	router.Use(AuthMiddleware())
	router.Use(CORSMiddleware())
	// Define a POST route for login
	router.POST("/login", LoginHandler(db))
	router.POST("/register", RegisterHandler(db))
	router.GET("/userCurrency/:id", GetUserCurrencyHandler(db))
	router.POST("/userCurrency", AddUserCurrencyHandler(db))
	router.POST("/updateUserCurrency", UpdateUserCurrencyHandler(db))

	// 应用事务中间件和分布式锁中间件
	router.POST("/addCurrencyNum", TransactionMiddleware(db), DistributedLockMiddleware(db, "add_currency_lock", 10*time.Second), AddCurrencyNumHandler(db))
	router.POST("/subtractCurrencyNum", TransactionMiddleware(db), DistributedLockMiddleware(db, "subtract_currency_lock", 10*time.Second), SubtractCurrencyNumHandler(db))

	if err := router.Run(":8080"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}

}
