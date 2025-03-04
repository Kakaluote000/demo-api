package main

import (
	"github.com/kakaluote000/demo-api/cmd/app"
	_ "github.com/kakaluote000/demo-api/docs"
	"github.com/kakaluote000/demo-api/internal/routes"
	"github.com/kakaluote000/demo-api/pkg"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title Currency Management System API
// @version 1.0
// @description 高性能虚拟货币管理系统
// @host localhost:8080
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
func main() {
	pkg.InitConfig()
	app := app.NewApp()
	routes.SetupRoutes(app)

	// 添加 Swagger 路由
	app.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	app.Run()
}
