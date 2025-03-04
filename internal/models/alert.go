package models

import (
	"time"
)

type AlertHistory struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	AlertName    string    `json:"alert_name"`
	Severity     string    `json:"severity"`
	Status       string    `json:"status"`
	Description  string    `json:"description"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time,omitempty"`
	HandledBy    string    `json:"handled_by,omitempty"`
	HandleStatus string    `json:"handle_status"`
	HandleNote   string    `json:"handle_note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}