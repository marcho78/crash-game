package models

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"time"
)

type GameHistory struct {
	GameID      string          `json:"game_id"`
	CrashPoint  float64         `json:"crash_point"`
	Hash        string          `json:"hash"`
	StartTime   time.Time       `json:"start_time"`
	EndTime     time.Time       `json:"end_time"`
	BetAmount   float64         `json:"bet_amount"`
	WinAmount   float64         `json:"win_amount"`
	CashedOut   bool            `json:"cashed_out"`
	CashoutAt   float64         `json:"cashout_at"`
	AutoCashout float64         `json:"auto_cashout"`
	Players     []PlayerHistory `json:"players,omitempty"`
	Status      string          `json:"status"`
}

type GameVerification struct {
	GameID string `json:"gameId" binding:"required"`
	Hash   string `json:"hash" binding:"required"`
}

type Player struct {
	UserID      string     `json:"userId"`
	BetAmount   float64    `json:"betAmount"`
	CashedOut   bool       `json:"cashedOut"`
	CashoutAt   *time.Time `json:"cashoutAt,omitempty"`
	WinAmount   float64    `json:"winAmount"`
	AutoCashout *float64   `json:"autoCashout,omitempty"`
}

type PlayerHistory struct {
	UserID      string     `json:"user_id"`
	BetAmount   float64    `json:"bet_amount"`
	WinAmount   float64    `json:"win_amount"`
	CashedOut   bool       `json:"cashed_out"`
	CashoutAt   *time.Time `json:"cashout_at"`
	AutoCashout *float64   `json:"auto_cashout"`
}

func CalculateCrashPoint(seed string) float64 {
	// Convert hex seed to bytes
	hash := sha256.Sum256([]byte(seed))

	// Use first 8 bytes as uint64
	num := binary.BigEndian.Uint64(hash[:8])

	// Generate float between 1 and 10
	float := 1.0 + (float64(num)/float64(1<<64))*9.0

	return math.Floor(float*100) / 100 // Round to 2 decimal places
}
