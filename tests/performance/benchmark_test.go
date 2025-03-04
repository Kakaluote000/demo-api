package performance

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/handlers"
	"github.com/kakaluote000/demo-api/internal/middleware"
)

func BenchmarkLoginHandler(b *testing.B) {
	gin.SetMode(gin.TestMode)
	app := app.NewApp()
	router := gin.New()
	router.POST("/login", handlers.LoginHandler(app))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/login",
			strings.NewReader(`{"username":"test","password":"test"}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
	}
}

func BenchmarkGetUserCurrency(b *testing.B) {
	gin.SetMode(gin.TestMode)
	app := app.NewApp()
	router := gin.New()
	router.GET("/userCurrency/:id", handlers.GetUserCurrencyHandler(app))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/userCurrency/1", nil)
		router.ServeHTTP(w, req)
	}
}

func BenchmarkAddCurrencyNum(b *testing.B) {
	gin.SetMode(gin.TestMode)
	app := app.NewApp()
	router := gin.New()
	router.POST("/addCurrencyNum", handlers.AddCurrencyNumHandler(app))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/addCurrencyNum",
			strings.NewReader(`{"user_id":1,"currency_id":1,"amount":100}`))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
	}
}

func BenchmarkConcurrentRequests(b *testing.B) {
	gin.SetMode(gin.TestMode)
	app := app.NewApp()
	router := gin.New()
	router.POST("/addCurrencyNum", handlers.AddCurrencyNumHandler(app))

	b.SetParallelism(100) // 设置并发数
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/addCurrencyNum",
				strings.NewReader(`{"user_id":1,"currency_id":1,"amount":100}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
		}
	})
}

func BenchmarkWithRedisCache(b *testing.B) {
	gin.SetMode(gin.TestMode)
	app := app.NewApp()
	router := gin.New()
	router.GET("/userCurrency/:id", handlers.GetUserCurrencyHandler(app))

	// 预热缓存
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/userCurrency/1", nil)
	router.ServeHTTP(w, req)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/userCurrency/1", nil)
			router.ServeHTTP(w, req)
		}
	})
}

func BenchmarkDistributedLock(b *testing.B) {
	gin.SetMode(gin.TestMode)
	app := app.NewApp()
	router := gin.New()
	router.POST("/updateUserCurrency",
		middleware.DistributedLockMiddleware(app, "test_lock", 1*time.Second),
		handlers.UpdateUserCurrencyHandler(app))

	b.SetParallelism(50)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/updateUserCurrency",
				strings.NewReader(`{"user_id":1,"currency_id":1,"currency_num":100}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
		}
	})
}
