package models

import "time"

type PaymentMethod struct {
	ID        int       `json:"id"`
	UserID    string    `json:"userId"`
	Type      string    `json:"type"`
	Address   string    `json:"address"`
	Label     string    `json:"label"`
	IsDefault bool      `json:"isDefault"`
	CreatedAt time.Time `json:"createdAt"`
}

type WithdrawalRequest struct {
	ID              int       `json:"id"`
	UserID          string    `json:"userId"`
	Amount          float64   `json:"amount"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"createdAt"`
	Username        string    `json:"username"`
	PaymentType     string    `json:"paymentType"`
	PaymentAddress  string    `json:"paymentAddress"`
	PaymentMethodID int       `json:"paymentMethodId"`
}

type DepositRequest struct {
	Amount          float64 `json:"amount" binding:"required,gt=0"`
	PaymentMethodID int     `json:"paymentMethodId" binding:"required"`
}

type UserSettings struct {
	UserID             string  `json:"userId"`
	Theme              string  `json:"theme"`
	SoundEnabled       bool    `json:"soundEnabled"`
	EmailNotifications bool    `json:"emailNotifications"`
	AutoCashoutEnabled bool    `json:"autoCashoutEnabled"`
	AutoCashoutValue   float64 `json:"autoCashoutValue"`
	Language           string  `json:"language"`
	Timezone           string  `json:"timezone"`
}
