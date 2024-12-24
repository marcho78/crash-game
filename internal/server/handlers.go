package server

import (
	"crash-game/internal/game"
	"crash-game/internal/models"

	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *GameServer) GetGameHistory(c *gin.Context) {
	userID := c.GetString("userId")

	history, err := s.db.GetGameHistory(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get game history"})
		return
	}

	c.JSON(200, gin.H{"history": history})
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
	log.Printf("Placing bet for user %s, amount: %f", userID, req.Amount)

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

	// Add debug logging for game state
	log.Printf("Current game status: %v", s.currentGame.Status)

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
	log.Printf("Starting withdrawal request for user: %s", userID)

	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid request: %v", err)
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	log.Printf("Withdrawal amount requested: %f", req.Amount)

	// Check balance first
	balance, err := s.db.GetUserBalance(userID)
	if err != nil {
		log.Printf("Failed to get balance: %v", err)
		c.JSON(500, gin.H{"error": "failed to get balance"})
		return
	}
	log.Printf("Current balance: %f", balance)

	if balance < req.Amount {
		log.Printf("Insufficient balance: %f < %f", balance, req.Amount)
		c.JSON(400, gin.H{"error": "insufficient balance"})
		return
	}

	withdrawalID := uuid.New().String()
	log.Printf("Generated withdrawal ID: %s", withdrawalID)

	if err := s.db.CreateWithdrawal(userID, &models.Withdrawal{
		ID:     withdrawalID,
		UserID: userID,
		Amount: req.Amount,
		Status: "pending",
	}); err != nil {
		log.Printf("Failed to create withdrawal: %v", err)
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Withdrawal request successful")
	c.JSON(200, gin.H{
		"id":     withdrawalID,
		"status": "pending",
		"amount": req.Amount,
	})
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

	s.mu.RLock()
	game := s.currentGame
	s.mu.RUnlock()

	if game == nil || game.Status != "in_progress" {
		c.JSON(400, gin.H{"error": "no active game"})
		return
	}

	multiplier := game.GetCurrentMultiplier()
	if err := game.PlayerCashout(userID, multiplier); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":    true,
		"multiplier": multiplier,
	})
}

func (s *GameServer) GetCurrentGame(c *gin.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.currentGame == nil {
		c.JSON(404, gin.H{"error": "no active game"})
		return
	}

	c.JSON(200, gin.H{
		"gameId": s.currentGame.GameID,
		"status": s.currentGame.Status,
		"hash":   s.currentGame.Hash,
	})
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
		GameID string `json:"gameId" binding:"required"`
		Hash   string `json:"hash" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("❌ VERIFY: Invalid JSON: %v", err)
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	// Check current game first
	s.mu.RLock()
	if s.currentGame != nil && s.currentGame.GameID == req.GameID {
		if s.currentGame.Hash == req.Hash {
			seed := s.currentGame.Hash[:8]
			crashPoint := game.CalculateCrashPoint(seed)
			s.mu.RUnlock()
			c.JSON(200, gin.H{
				"valid":      true,
				"crashPoint": crashPoint,
				"seed":       seed,
			})
			return
		}
	}
	s.mu.RUnlock()

	// If not current game, check database
	gameData, err := s.db.GetGameByID(req.GameID)
	if err != nil {
		log.Printf("❌ VERIFY: DB lookup failed: %v", err)
		c.JSON(404, gin.H{"error": "game not found"})
		return
	}

	if gameData.Hash != req.Hash {
		c.JSON(400, gin.H{"error": "invalid hash"})
		return
	}

	seed := gameData.Hash[:8]
	crashPoint := game.CalculateCrashPoint(seed)
	c.JSON(200, gin.H{
		"valid":      true,
		"crashPoint": crashPoint,
		"seed":       seed,
	})
}
