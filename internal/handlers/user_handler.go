package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/models"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

func RegisterHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := app.DB
		var user models.User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 检查用户名是否已经存在
		var existingUser models.User
		if err := db.Where("username = ?", user.Username).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Username already exists"})
			return
		}

		if err := db.Create(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "User registered successfully"})
	}
}

func LoginHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := app.DB
		var loginReq LoginRequest
		if err := c.ShouldBindJSON(&loginReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var user models.User
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

func AddUserCurrencyHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := app.DB
		rdb := app.Redis
		var userCurrency models.UserCurrency
		if err := c.ShouldBindJSON(&userCurrency); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 检查用户是否存在
		var user models.User
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
		rdb.Del(app.Ctx, cacheKey)

		c.JSON(http.StatusOK, gin.H{"message": "User currency added successfully"})
	}
}

func GetUserCurrencyHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := app.DB
		rdb := app.Redis
		id := c.Param("id")
		cacheKey := fmt.Sprintf("user_currency:%s", id)

		// 尝试从缓存中获取数据
		cachedData, err := rdb.Get(app.Ctx, cacheKey).Result()
		if err == nil {
			var userCurrency models.UserCurrency
			if err := json.Unmarshal([]byte(cachedData), &userCurrency); err == nil {
				c.JSON(http.StatusOK, userCurrency)
				return
			}
		}

		// 如果缓存中没有数据，则从数据库中获取
		var userCurrency models.UserCurrency
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
			rdb.Set(app.Ctx, cacheKey, data, time.Hour) // 缓存有效期为1小时
		}

		c.JSON(http.StatusOK, userCurrency)
	}
}

func UpdateUserCurrencyHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := app.DB
		rdb := app.Redis
		var userCurrency models.UserCurrency
		if err := c.ShouldBindJSON(&userCurrency); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 检查用户是否存在
		var user models.User
		if err := db.Where("id = ?", userCurrency.UserID).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 更新用户货币记录
		if err := db.Model(&models.UserCurrency{}).Where("user_id = ? AND currency_id = ?", userCurrency.UserID, userCurrency.CurrencyID).Updates(models.UserCurrency{
			CurrencyNum: userCurrency.CurrencyNum,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user currency"})
			return
		}

		// 清除缓存
		cacheKey := fmt.Sprintf("user_currency:%d", userCurrency.UserID)
		rdb.Del(app.Ctx, cacheKey)

		c.JSON(http.StatusOK, gin.H{"message": "User currency updated successfully"})
	}
}

func AddCurrencyNumHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := app.DB
		rdb := app.Redis
		// 从上下文中获取解析后的userCurrency
		val, ok := c.Get("userCurrency")
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid userCurrency"})
			return
		}

		userCurrency := val.(models.UserCurrency)

		// 检查用户是否存在
		var user models.User
		if err := db.Where("id = ?", userCurrency.UserID).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 检查用户货币记录是否存在
		var existingUserCurrency models.UserCurrency
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
		if err := db.Model(&models.UserCurrency{}).Where("user_id = ? AND currency_id = ?", userCurrency.UserID, userCurrency.CurrencyID).Updates(models.UserCurrency{
			CurrencyNum: newCurrencyNum,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user currency"})
			return
		}

		// 记录流水日志
		transaction := models.CurrencyTransaction{
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
		rdb.Del(app.Ctx, cacheKey)

		c.JSON(http.StatusOK, gin.H{"message": "User currency added successfully", "new_currency_num": newCurrencyNum})
	}
}

func SubtractCurrencyNumHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := app.DB
		rdb := app.Redis
		// 从上下文中获取解析后的userCurrency
		val, ok := c.Get("userCurrency")
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid userCurrency"})
			return
		}

		userCurrency := val.(models.UserCurrency)

		// 检查用户是否存在
		var user models.User
		if err := db.Where("id = ?", userCurrency.UserID).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 检查用户货币记录是否存在
		var existingUserCurrency models.UserCurrency
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
		if err := db.Model(&models.UserCurrency{}).Where("user_id = ? AND currency_id = ?", userCurrency.UserID, userCurrency.CurrencyID).Updates(models.UserCurrency{
			CurrencyNum: newCurrencyNum}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user currency"})
			return
		}

		// 记录流水日志
		transaction := models.CurrencyTransaction{
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
		rdb.Del(app.Ctx, cacheKey)

		c.JSON(http.StatusOK, gin.H{"message": "User currency subtracted successfully", "new_currency_num": newCurrencyNum})
	}
}
