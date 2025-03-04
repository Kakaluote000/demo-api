package models

import (
	"time"
)

type AlertRule struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	AlertName       string    `json:"alert_name"`
	Severity        string    `json:"severity"`
	AutoHandle      bool      `json:"auto_handle"`
	AutoHandleRule  string    `json:"auto_handle_rule"`
	EscalationTime  int      `json:"escalation_time"` // 分钟
	EscalationLevel int      `json:"escalation_level"`
	NotifyUsers     string    `json:"notify_users"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}