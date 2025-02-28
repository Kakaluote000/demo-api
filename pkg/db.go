package pkg

import (
	"github.com/kakaluote000/demo-api/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB() *gorm.DB {
	dsn := "demo_db_user:demoAbc123@tcp(127.0.0.1:3306)/demo_db?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logrus.Fatalf("failed to connect database: %v", err)
		panic("failed to connect database")
	}
	return db
}

func AutoMigrate(db *gorm.DB) {
	db.AutoMigrate(&models.User{})
	db.AutoMigrate(&models.UserCurrency{})
	db.AutoMigrate(&models.CurrencyTransaction{})
}
