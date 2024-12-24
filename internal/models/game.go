package models

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"time"
)

type GameHistory struct {
	GameID            string          `json:"gameId"`
	CrashPoint        float64         `json:"crashPoint"`
	StartTime         time.Time       `json:"startTime"`
	EndTime           time.Time       `json:"endTime"`
	BetAmount         *float64        `json:"betAmount,omitempty"`
	CashoutMultiplier *float64        `json:"cashoutMultiplier,omitempty"`
	WinAmount         *float64        `json:"winAmount,omitempty"`
	Hash              string          `json:"hash"`
	Players           []PlayerHistory `json:"players"`
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
	UserID     string     `json:"userId"`
	BetAmount  float64    `json:"betAmount"`
	CashedOut  bool       `json:"cashedOut"`
	CashoutAt  *time.Time `json:"cashoutAt,omitempty"`
	WinAmount  float64    `json:"winAmount"`
	Multiplier float64    `json:"multiplier,omitempty"`
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
