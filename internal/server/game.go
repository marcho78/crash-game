package server

import (
	"errors"
	"math"
	"time"
)

func (g *GameState) PlayerCashout(userID string, multiplier float64) error {
	player, exists := g.Players[userID]
	if !exists {
		return errors.New("no bet found for this game")
	}
	if player.CashedOut {
		return errors.New("already cashed out")
	}

	player.CashedOut = true
	player.WinAmount = player.BetAmount * multiplier
	return nil
}

func (g *GameState) GetCurrentMultiplier() float64 {
	if g.Status != "in_progress" {
		return 0
	}
	elapsed := time.Since(g.StartTime).Seconds()
	return math.Floor(math.Pow(math.E, elapsed/8)*100) / 100
}
