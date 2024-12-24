package notification

import (
	"crash-game/internal/database"
	"crash-game/internal/models"
	"sync"
	"time"
)

type NotificationManager struct {
	notifications map[int]*models.AdminNotification
	subscribers   map[string]chan *models.AdminNotification
	mu            sync.RWMutex
	db            *database.Database
}

func NewNotificationManager(db *database.Database) *NotificationManager {
	nm := &NotificationManager{
		notifications: make(map[int]*models.AdminNotification),
		subscribers:   make(map[string]chan *models.AdminNotification),
		db:            db,
	}
	go nm.cleanupOldNotifications()
	return nm
}

func (nm *NotificationManager) Subscribe(adminID string) chan *models.AdminNotification {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	ch := make(chan *models.AdminNotification, 100)
	nm.subscribers[adminID] = ch
	return ch
}

func (nm *NotificationManager) Unsubscribe(adminID string) {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	if ch, exists := nm.subscribers[adminID]; exists {
		close(ch)
		delete(nm.subscribers, adminID)
	}
}

func (nm *NotificationManager) CreateNotification(notif *models.AdminNotification) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	// Save to database
	if err := nm.db.SaveNotification(notif); err != nil {
		return err
	}

	// Broadcast to all subscribers
	for _, ch := range nm.subscribers {
		select {
		case ch <- notif:
		default:
			// Channel is full, skip
		}
	}

	return nil
}

func (nm *NotificationManager) cleanupOldNotifications() {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		nm.db.CleanupOldNotifications(30) // Keep 30 days of notifications
	}
}
