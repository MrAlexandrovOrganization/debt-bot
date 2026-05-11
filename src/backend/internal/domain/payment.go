package domain

import "time"

type Payment struct {
	ID         string
	DealID     string
	FromUserID string
	ToUserID   string
	Amount     int64 // in kopecks
	CreatedAt  time.Time
}
