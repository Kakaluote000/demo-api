package main

import (
	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/routes"
	"github.com/kakaluote000/demo-api/pkg"
)

func main() {
	pkg.InitConfig()
	app := app.NewApp()
	routes.SetupRoutes(app)
	app.Run()
}
