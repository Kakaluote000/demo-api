package app

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/kakaluote000/demo-api/pkg"
	"gorm.io/gorm"
)

var GlobalApp *App

type App struct {
	DB     *gorm.DB
	Redis  *redis.Client
	RS     *redsync.Redsync
	Router *gin.Engine
	Ctx    context.Context
}

func NewApp() *App {
	db := pkg.InitDB()
	rdb, rs, ctx := pkg.InitRedis()

	GlobalApp = &App{
		DB:     db,
		Redis:  rdb,
		RS:     rs,
		Router: gin.Default(),
		Ctx:    ctx,
	}
	return GlobalApp
}

func (app *App) Run() {
	if err := app.Router.Run(":8080"); err != nil {
		pkg.Log.Fatalf("failed to run server: %v", err)
	}
}
