package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type Notifier interface {
	Send(message string) error
}

type WebhookNotifier struct {
	WebhookURL string
}

type DingTalkNotifier struct {
	AccessToken string
}

type EmailNotifier struct {
	SMTPServer string
	SMTPPort   int
	Username   string
	Password   string
	From       string
	To         []string
}

func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{WebhookURL: url}
}

func (n *WebhookNotifier) Send(message string) error {
	payload := map[string]string{"text": message}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(n.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook notification failed with status: %d", resp.StatusCode)
	}
	return nil
}
