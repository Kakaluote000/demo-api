package notification

import (
	"sync"
)

type NotificationManager struct {
	notifiers []Notifier
	mu        sync.RWMutex
}

func (m *NotificationManager) NotifyUser(user string, message string) {
	panic("unimplemented")
}

func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		notifiers: make([]Notifier, 0),
	}
}

func (m *NotificationManager) AddNotifier(n Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers = append(m.notifiers, n)
}

func (m *NotificationManager) NotifyAll(message string) []error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	errors := make([]error, 0)
	for _, notifier := range m.notifiers {
		if err := notifier.Send(message); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}
