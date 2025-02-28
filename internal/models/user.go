package models

import (
	"time"

	"gorm.io/gorm"
)

// User 定义用户模型，对应 user 表
type User struct {
	gorm.Model
	Username string `gorm:"column:username;not null;unique" json:"username"`
	Password string `gorm:"column:password;not null" json:"password"`
}

// UserCurrency 定义用户货币模型，对应 user_currency 表
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
