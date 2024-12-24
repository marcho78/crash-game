package server

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"crash-game/internal/game"
	"crash-game/internal/models"

	"log"

	"github.com/gin-gonic/gin"
)

func (s *GameServer) GetGameHistory(c *gin.Context) {
	limit := 20 // Default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	s.historyMu.RLock()
	historyLen := len(s.gameHistory)
	if limit > historyLen {
		limit = historyLen
	}
	history := s.gameHistory[:limit]
	s.historyMu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"history": history,
	})
}

func (s *GameServer) GetProfile(c *gin.Context) {
	userID := c.GetString("userId")
	profile, err := s.db.GetUserProfile(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get profile"})
		return
	}
	c.JSON(200, profile)
}

func (s *GameServer) UpdateBalance(c *gin.Context) {
	userID := c.GetString("userId")
	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
		Type   string  `json:"type" binding:"required,oneof=credit debit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	if err := s.db.UpdateBalance(userID, req.Amount, req.Type); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true})
}

func (s *GameServer) PlaceBet(c *gin.Context) {
	var req struct {
		Amount      float64  `json:"amount" binding:"required,gt=0"`
		AutoCashout *float64 `json:"autoCashout" binding:"omitempty,gt=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	userID := c.GetString("userId")

	// Check user balance
	balance, err := s.db.GetUserBalance(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get balance"})
		return
	}

	if balance < req.Amount {
		c.JSON(400, gin.H{"error": "insufficient balance"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate game state
	if s.currentGame == nil || s.currentGame.Status != "betting" {
		c.JSON(400, gin.H{"error": "game not accepting bets"})
		return
	}

	// Check for existing bet
	if _, exists := s.currentGame.Players[userID]; exists {
		c.JSON(400, gin.H{"error": "bet already placed for this game"})
		return
	}

	// Update balance first
	if err := s.db.UpdateBalance(userID, req.Amount, "debit"); err != nil {
		log.Printf("Failed to update balance: %v", err)
		c.JSON(500, gin.H{"error": "failed to update balance: " + err.Error()})
		return
	}

	// Record the bet
	s.currentGame.Players[userID] = &Player{
		BetAmount:   req.Amount,
		CashedOut:   false,
		AutoCashout: req.AutoCashout,
	}

	c.JSON(200, gin.H{
		"success": true,
		"amount":  req.Amount,
	})
}

func (s *GameServer) RequestWithdrawal(c *gin.Context) {
	userID := c.GetString("userId")
	var req models.WithdrawalRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	if err := s.db.CreateWithdrawal(userID, &req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "withdrawal request created"})
}

func (s *GameServer) RequestDeposit(c *gin.Context) {
	userID := c.GetString("userId")
	var req models.DepositRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	address, err := s.db.CreateDeposit(userID, &req)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"message": "deposit request created",
		"address": address,
	})
}

func (s *GameServer) GetSettings(c *gin.Context) {
	userID := c.GetString("userId")

	settings, err := s.db.GetUserSettings(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, settings)
}

func (s *GameServer) UpdateSettings(c *gin.Context) {
	userID := c.GetString("userId")
	var settings models.UserSettings

	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	settings.UserID = userID
	if err := s.db.UpdateUserSettings(userID, &settings); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "settings updated"})
}

func (s *GameServer) Cashout(c *gin.Context) {
	userID := c.GetString("userId")

	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("Cashout attempt - Game State: %+v", s.currentGame)

	if s.currentGame == nil || s.currentGame.Status != "in_progress" {
		log.Printf("Game not in progress - Status: %v", s.currentGame.Status)
		c.JSON(400, gin.H{"error": "no active game"})
		return
	}

	player, exists := s.currentGame.Players[userID]
	log.Printf("Player exists: %v, Player state: %+v", exists, player)

	if !exists || player.CashedOut {
		c.JSON(400, gin.H{"error": "no active bet or already cashed out"})
		return
	}

	elapsed := time.Since(s.currentGame.StartTime).Seconds()
	multiplier := math.Pow(math.E, 0.1*elapsed)
	winAmount := player.BetAmount * multiplier

	now := time.Now()
	player.CashedOut = true
	player.CashoutAt = &now
	player.WinAmount = winAmount

	err := s.db.UpdateBalance(userID, winAmount, "credit")
	if err != nil {
		log.Printf("Error updating balance: %v", err)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Cashout successful - Win Amount: %f", winAmount)

	c.JSON(200, gin.H{
		"success":    true,
		"multiplier": multiplier,
		"winAmount":  winAmount,
	})
}

func (s *GameServer) GetCurrentGame(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c.JSON(200, s.currentGame)
}

func (s *GameServer) GetBalance(c *gin.Context) {
	userID := c.GetString("userId")

	balance, err := s.db.GetUserBalance(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"balance": balance,
	})
}

func (s *GameServer) VerifyGameFairness(c *gin.Context) {
	var req struct {
		GameID string `json:"gameId"`
		Hash   string `json:"hash"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid input"})
		return
	}

	result := game.VerifyGameHash(req.GameID, req.Hash)
	c.JSON(200, gin.H{
		"valid":              result.Valid,
		"expectedCrashPoint": result.ExpectedCrashPoint,
		"seed":               result.Seed,
	})
}
