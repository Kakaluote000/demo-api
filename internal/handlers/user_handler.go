package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/models"
	"github.com/kakaluote000/demo-api/pkg/auth"
	"github.com/kakaluote000/demo-api/pkg/security"
	"gorm.io/gorm"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// RegisterHandler godoc
// @Summary 用户注册
// @Description 注册新用户
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param user body models.User true "用户信息"
// @Success 200 {object} response.SuccessResponse
// @Failure 400 {object} response.ErrorResponse
// @Router /register [post]
func RegisterHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var user models.User
		if err := c.ShouldBindJSON(&user); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 验证密码强度
		if !security.ValidatePassword(user.Password) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password does not meet security requirements"})
			return
		}

		// 加密密码
		hashedPassword, err := security.HashPassword(user.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
			return
		}
		user.Password = hashedPassword

		db := app.DB
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

// LoginHandler godoc
// @Summary 用户登录
// @Description 用户登录并返回token
// @Tags 用户管理
// @Accept json
// @Produce json
// @Param user body LoginRequest true "登录信息"
// @Success 200 {object} response.SuccessResponse
// @Failure 400,401 {object} response.ErrorResponse
// @Router /login [post]
func LoginHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		db := app.DB
		var loginReq LoginRequest
		if err := c.ShouldBindJSON(&loginReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var user models.User
		if err := db.Where("username = ?", loginReq.Username).First(&user).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			return
		}

		// 验证密码
		if !security.CheckPasswordHash(loginReq.Password, user.Password) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
			return
		}

		// 生成 JWT token
		token, err := auth.GenerateToken(user.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
			return
		}

		// 将 token 存入 Redis，用于后续验证
		tokenKey := fmt.Sprintf("user_token:%d", user.ID)
		err = app.Redis.Set(app.Ctx, tokenKey, token, 24*time.Hour).Err()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save token"})
			return
		}

		c.JSON(http.StatusOK, LoginResponse{
			Token: token,
		})
	}
}

// AddUserCurrencyHandler godoc
// @Summary 添加用户货币
// @Description 为用户添加新的货币类型
// @Tags 货币管理
// @Accept json
// @Produce json
// @Param userCurrency body models.UserCurrency true "用户货币信息"
// @Success 200 {object} response.SuccessResponse
// @Failure 400,404,500 {object} response.ErrorResponse
// @Security Bearer
// @Router /userCurrency [post]
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

// GetUserCurrencyHandler godoc
// @Summary 获取用户货币
// @Description 获取指定用户的货币信息
// @Tags 货币管理
// @Accept json
// @Produce json
// @Param id path int true "用户ID"
// @Success 200 {object} models.UserCurrency
// @Failure 404,500 {object} response.ErrorResponse
// @Security Bearer
// @Router /userCurrency/{id} [get]
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

// UpdateUserCurrencyHandler godoc
// @Summary 更新用户货币
// @Description 更新用户的货币数量
// @Tags 货币管理
// @Accept json
// @Produce json
// @Param userCurrency body models.UserCurrency true "用户货币信息"
// @Success 200 {object} response.SuccessResponse
// @Failure 400,404,500 {object} response.ErrorResponse
// @Security Bearer
// @Router /userCurrency [put]
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

// AddCurrencyNumHandler godoc
// @Summary 增加货币数量
// @Description 增加用户的货币数量
// @Tags 货币管理
// @Accept json
// @Produce json
// @Param userCurrency body models.UserCurrency true "用户货币信息"
// @Success 200 {object} response.SuccessResponse
// @Failure 400,404,500 {object} response.ErrorResponse
// @Security Bearer
// @Router /addCurrencyNum [post]
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

// SubtractCurrencyNumHandler godoc
// @Summary 减少货币数量
// @Description 减少用户的货币数量
// @Tags 货币管理
// @Accept json
// @Produce json
// @Param userCurrency body models.UserCurrency true "用户货币信息"
// @Success 200 {object} response.SuccessResponse
// @Failure 400,404,500 {object} response.ErrorResponse
// @Security Bearer
// @Router /subtractCurrencyNum [post]
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
