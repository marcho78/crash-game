package models

import (
	"time"
)

type GameHistory struct {
	GameID     string          `json:"gameId"`
	CrashPoint float64         `json:"crashPoint"`
	StartTime  time.Time       `json:"startTime"`
	EndTime    time.Time       `json:"endTime"`
	Hash       string          `json:"hash"`
	Players    []PlayerHistory `json:"players"`
}

type Player struct {
	BetAmount  float64    `json:"betAmount"`
	CashedOut  bool       `json:"cashedOut"`
	CashoutAt  *time.Time `json:"cashoutAt"`
	WinAmount  float64    `json:"winAmount"`
	Multiplier float64    `json:"multiplier"`
}

type PlayerHistory struct {
	UserID     string     `json:"userId"`
	BetAmount  float64    `json:"betAmount"`
	CashedOut  bool       `json:"cashedOut"`
	CashoutAt  *time.Time `json:"cashoutAt,omitempty"`
	WinAmount  float64    `json:"winAmount"`
	Multiplier float64    `json:"multiplier"`
}
