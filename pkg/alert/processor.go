package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kakaluote000/demo-api/internal/models"
	"github.com/kakaluote000/demo-api/pkg/notification"
	"gorm.io/gorm"
)

type AlertProcessor struct {
	db        *gorm.DB
	notifier  *notification.NotificationManager
	ruleCache map[string]*models.AlertRule
}

func NewAlertProcessor(db *gorm.DB, notifier *notification.NotificationManager) *AlertProcessor {
	return &AlertProcessor{
		db:        db,
		notifier:  notifier,
		ruleCache: make(map[string]*models.AlertRule),
	}
}

func (p *AlertProcessor) ProcessAlert(alert *models.AlertHistory) error {
	rule := p.getAlertRule(alert.AlertName)
	if rule == nil {
		return nil
	}

	if rule.AutoHandle {
		return p.handleAlertAutomatically(alert, rule)
	}

	if p.shouldEscalate(alert, rule) {
		return p.escalateAlert(alert, rule)
	}

	return nil
}

func (p *AlertProcessor) handleAlertAutomatically(alert *models.AlertHistory, rule *models.AlertRule) error {
	// 根据规则执行自动处理
	alert.HandleStatus = "auto_handled"
	alert.HandledBy = "system"
	alert.HandleNote = "自动处理完成"

	return p.db.Save(alert).Error
}

func (p *AlertProcessor) shouldEscalate(alert *models.AlertHistory, rule *models.AlertRule) bool {
	if alert.HandleStatus != "pending" {
		return false
	}

	escalationDuration := time.Duration(rule.EscalationTime) * time.Minute
	return time.Since(alert.CreatedAt) > escalationDuration
}

func (p *AlertProcessor) escalateAlert(alert *models.AlertHistory, rule *models.AlertRule) error {
	alert.HandleStatus = "escalated"

	var notifyUsers []string
	json.Unmarshal([]byte(rule.NotifyUsers), &notifyUsers)

	message := fmt.Sprintf("告警升级通知\n告警名称: %s\n严重程度: %s\n描述: %s",
		alert.AlertName,
		alert.Severity,
		alert.Description)

	for _, user := range notifyUsers {
		p.notifier.NotifyUser(user, message)
	}

	return p.db.Save(alert).Error
}

func (p *AlertProcessor) getAlertRule(alertName string) *models.AlertRule {
	if rule, ok := p.ruleCache[alertName]; ok {
		return rule
	}

	var rule models.AlertRule
	if err := p.db.Where("alert_name = ?", alertName).First(&rule).Error; err != nil {
		return nil
	}

	p.ruleCache[alertName] = &rule
	return &rule
}
