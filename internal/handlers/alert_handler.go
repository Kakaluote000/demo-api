package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kakaluote000/demo-api/cmd/app"
	"github.com/kakaluote000/demo-api/internal/models"
	"github.com/kakaluote000/demo-api/pkg/alert"
	"github.com/kakaluote000/demo-api/pkg/notification"
	"github.com/sirupsen/logrus"
)

type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    string            `json:"startsAt"`
	EndsAt      string            `json:"endsAt"`
}

type AlertWebhook struct {
	Version           string            `json:"version"`
	GroupKey          string            `json:"groupKey"`
	Status            string            `json:"status"`
	Receiver          string            `json:"receiver"`
	GroupLabels       map[string]string `json:"groupLabels"`
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	ExternalURL       string            `json:"externalURL"`
	Alerts            []Alert           `json:"alerts"`
}

func AlertWebhookHandler(app *app.App) gin.HandlerFunc {
	notificationManager := notification.NewNotificationManager()
	notificationManager.AddNotifier(notification.NewWebhookNotifier("https://your-webhook-url"))
	alertProcessor := alert.NewAlertProcessor(app.DB, notificationManager)

	return func(c *gin.Context) {
		var webhook AlertWebhook
		if err := c.ShouldBindJSON(&webhook); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 记录告警信息
		log := app.Log.WithFields(logrus.Fields{
			"status":   webhook.Status,
			"receiver": webhook.Receiver,
			"alerts":   len(webhook.Alerts),
		})

		// 处理每个告警
		for _, alert := range webhook.Alerts {
			// 将告警信息存入Redis用于统计
			alertKey := fmt.Sprintf("alert:%s:%s", alert.Labels["alertname"], alert.StartsAt)
			alertData, _ := json.Marshal(alert)
			app.Redis.Set(app.Ctx, alertKey, alertData, 24*time.Hour)

			log.WithFields(logrus.Fields{
				"alertname": alert.Labels["alertname"],
				"severity":  alert.Labels["severity"],
				"summary":   alert.Annotations["summary"],
			}).Info("Received alert")
		}

		// 发送通知
		for _, alert := range webhook.Alerts {
			message := fmt.Sprintf("告警: %s\n严重程度: %s\n描述: %s",
				alert.Labels["alertname"],
				alert.Labels["severity"],
				alert.Annotations["description"])

			if errs := notificationManager.NotifyAll(message); len(errs) > 0 {
				log.WithField("errors", errs).Error("Failed to send notifications")
			}
		}

		// 处理每个告警并记录历史
		for _, alert := range webhook.Alerts {
			startTime, _ := time.Parse(time.RFC3339, alert.StartsAt)
			endTime, _ := time.Parse(time.RFC3339, alert.EndsAt)

			alertHistory := models.AlertHistory{
				AlertName:    alert.Labels["alertname"],
				Severity:     alert.Labels["severity"],
				Status:       alert.Status,
				Description:  alert.Annotations["description"],
				StartTime:    startTime,
				EndTime:      endTime,
				HandleStatus: "pending",
			}

			if err := app.DB.Create(&alertHistory).Error; err != nil {
				log.WithError(err).Error("Failed to save alert history")
				continue
			}

			// 处理告警
			if err := alertProcessor.ProcessAlert(&alertHistory); err != nil {
				log.WithError(err).Error("Failed to process alert")
			}
		}

		c.JSON(http.StatusOK, gin.H{"message": "Alert received and processed"})
	}
}

// 添加获取告警历史的处理器
func GetAlertHistoryHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var alerts []models.AlertHistory
		query := app.DB.Order("created_at desc")

		// 支持按状态筛选
		if status := c.Query("status"); status != "" {
			query = query.Where("handle_status = ?", status)
		}

		// 支持按严重程度筛选
		if severity := c.Query("severity"); severity != "" {
			query = query.Where("severity = ?", severity)
		}

		// 支持时间范围筛选
		if startDate := c.Query("start_date"); startDate != "" {
			query = query.Where("start_time >= ?", startDate)
		}
		if endDate := c.Query("end_date"); endDate != "" {
			query = query.Where("start_time <= ?", endDate)
		}

		if err := query.Find(&alerts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch alert history"})
			return
		}

		c.JSON(http.StatusOK, alerts)
	}
}

// 添加更新告警处理状态的处理器
func UpdateAlertStatusHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		alertID := c.Param("id")
		var updateData struct {
			HandleStatus string `json:"handle_status"`
			HandleNote   string `json:"handle_note"`
			HandledBy    string `json:"handled_by"`
		}

		if err := c.ShouldBindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result := app.DB.Model(&models.AlertHistory{}).
			Where("id = ?", alertID).
			Updates(map[string]interface{}{
				"handle_status": updateData.HandleStatus,
				"handle_note":   updateData.HandleNote,
				"handled_by":    updateData.HandledBy,
			})

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update alert status"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Alert status updated successfully"})
	}
}

func GetAlertStatsHandler(app *app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 获取最近24小时的告警统计
		now := time.Now()
		now = now.Add(-24 * time.Hour)

		var stats struct {
			Total    int            `json:"total"`
			Critical int            `json:"critical"`
			Warning  int            `json:"warning"`
			ByType   map[string]int `json:"by_type"`
			Resolved int            `json:"resolved"`
			Active   int            `json:"active"`
		}
		stats.ByType = make(map[string]int)

		// 扫描所有告警
		pattern := "alert:*"
		iter := app.Redis.Scan(app.Ctx, 0, pattern, 0).Iterator()
		for iter.Next(app.Ctx) {
			key := iter.Val()
			alertData, err := app.Redis.Get(app.Ctx, key).Result()
			if err != nil {
				continue
			}

			var alert Alert
			if err := json.Unmarshal([]byte(alertData), &alert); err != nil {
				continue
			}

			// 统计告警信息
			stats.Total++
			if alert.Labels["severity"] == "critical" {
				stats.Critical++
			} else if alert.Labels["severity"] == "warning" {
				stats.Warning++
			}

			alertType := alert.Labels["alertname"]
			stats.ByType[alertType]++

			if alert.EndsAt != "" {
				stats.Resolved++
			} else {
				stats.Active++
			}
		}

		c.JSON(http.StatusOK, stats)
	}
}
