package server

import (
	"crash-game/internal/game"
	"crash-game/internal/models"

	"fmt"
	"log"
	"strings"
	"time"

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
		AutoCashout *float64 `json:"auto_cashout" binding:"omitempty,gt=1"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		if strings.Contains(err.Error(), "failed on the 'gt' tag") {
			c.JSON(400, gin.H{"error": "invalid amount: must be greater than 0"})
			return
		}
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	userID := c.GetString("userId")
	log.Printf("Placing bet for user %s, amount: %f", userID, req.Amount)

	// Check balance first
	balance, err := s.db.GetUserBalance(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get balance"})
		return
	}

	// Check insufficient balance before max bet
	if balance < req.Amount {
		c.JSON(400, gin.H{"error": "insufficient balance"})
		return
	}

	// Then check maximum bet
	const maxBetAmount = 1000.0
	if req.Amount > maxBetAmount {
		c.JSON(400, gin.H{"error": "bet amount exceeds maximum allowed"})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate game state BEFORE deducting balance
	if s.currentGame == nil || s.currentGame.Status != "betting" {
		c.JSON(400, gin.H{"error": "game not accepting bets"})
		return
	}

	// Check for existing bet
	if _, exists := s.currentGame.Players[userID]; exists {
		c.JSON(400, gin.H{"error": "bet already placed for this game"})
		return
	}

	// Deduct bet amount ONLY after all validations pass
	if err := s.db.UpdateBalance(userID, req.Amount, "debit"); err != nil {
		log.Printf("❌ BET: Failed to update balance: %v", err)
		c.JSON(500, gin.H{"error": "failed to update balance"})
		return
	}

	// Record the bet
	log.Printf("📝 DEBUG: Recording bet - User: %s, Amount: %.2f, AutoCashout: %v",
		userID, req.Amount, req.AutoCashout)

	autoCashoutValue := "<nil>"
	if req.AutoCashout != nil {
		autoCashoutValue = fmt.Sprintf("%.2f", *req.AutoCashout)
	}
	log.Printf("🎯 DEBUG: [BET] Setting up bet - User: %s, Amount: %.2f, AutoCashout: %s",
		userID, req.Amount, autoCashoutValue)

	s.currentGame.Players[userID] = &Player{
		UserID:      userID,
		BetAmount:   req.Amount,
		CashedOut:   false,
		WinAmount:   0,
		AutoCashout: req.AutoCashout,
	}

	// Verify the bet was recorded correctly
	if player := s.currentGame.Players[userID]; player != nil {
		actualAutoCashout := "<nil>"
		if player.AutoCashout != nil {
			actualAutoCashout = fmt.Sprintf("%.2f", *player.AutoCashout)
		}
		log.Printf("✅ DEBUG: [BET] Bet recorded - User: %s, AutoCashout: %s",
			userID, actualAutoCashout)
	}

	if req.AutoCashout != nil {
		log.Printf("🎯 DEBUG: Auto-cashout set to %.2fx for user %s", *req.AutoCashout, userID)
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

	// Deduct from balance
	if err := s.db.UpdateBalance(userID, req.Amount, "debit"); err != nil {
		log.Printf("Failed to update balance: %v", err)
		c.JSON(500, gin.H{"error": "failed to update balance"})
		return
	}

	withdrawalID := uuid.New().String()
	log.Printf("Generated withdrawal ID: %s", withdrawalID)

	// Create withdrawal record
	if err := s.db.CreateWithdrawal(userID, &models.Withdrawal{
		ID:     withdrawalID,
		UserID: userID,
		Amount: req.Amount,
		Status: "pending",
	}); err != nil {
		// Rollback balance if withdrawal creation fails
		s.db.UpdateBalance(userID, req.Amount, "credit")
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

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentGame == nil || s.currentGame.Status != "in_progress" {
		c.JSON(400, gin.H{"error": "no active game"})
		return
	}

	player, exists := s.currentGame.Players[userID]
	if !exists {
		c.JSON(400, gin.H{"error": "no bet found for this game"})
		return
	}

	if player.CashedOut {
		c.JSON(400, gin.H{"error": "already cashed out"})
		return
	}

	multiplier := s.currentGame.GetCurrentMultiplier()
	winAmount := player.BetAmount * multiplier

	// Update player state
	player.CashedOut = true
	player.WinAmount = winAmount
	now := time.Now()
	player.CashoutAt = &now

	// Add debug logging
	initialBalance, _ := s.db.GetUserBalance(userID)
	log.Printf("DEBUG: Balance before cashout: %f", initialBalance)

	// Update user balance with winnings
	if err := s.db.UpdateBalance(userID, winAmount, "credit"); err != nil {
		c.JSON(500, gin.H{"error": "failed to update balance"})
		return
	}

	// Verify credit
	finalBalance, _ := s.db.GetUserBalance(userID)
	log.Printf("DEBUG: Balance after cashout: %f (win amount: %f)", finalBalance, winAmount)

	c.JSON(200, gin.H{
		"success":    true,
		"multiplier": multiplier,
		"winAmount":  winAmount,
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
		"gameId":  s.currentGame.GameID,
		"status":  s.currentGame.Status,
		"hash":    s.currentGame.Hash,
		"players": s.currentGame.Players,
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

func (s *GameServer) GetPlayerGameHistory(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		log.Printf("❌ Unauthorized access attempt to player history")
		c.JSON(401, gin.H{"error": "unauthorized"})
		return
	}

	log.Printf("📊 Getting bet history for user: %s", userID)

	rows, err := s.db.GetPlayerBetHistory(userID)
	if err != nil {
		log.Printf("❌ Database query failed: %v", err)
		c.JSON(500, gin.H{"error": "failed to get bet history"})
		return
	}
	defer rows.Close()

	var history []gin.H
	for rows.Next() {
		var h struct {
			GameID            string     `json:"game_id"`
			BetAmount         float64    `json:"bet_amount"`
			WinAmount         *float64   `json:"win_amount"`
			CashedOut         bool       `json:"cashed_out"`
			CashoutMultiplier *float64   `json:"cashout_multiplier"`
			AutoCashout       *float64   `json:"auto_cashout"`
			CreatedAt         time.Time  `json:"created_at"`
			CashoutAt         *time.Time `json:"cashout_at"`
			CrashPoint        float64    `json:"crash_point"`
			Hash              string     `json:"hash"`
			Status            string     `json:"status"`
		}

		err := rows.Scan(
			&h.GameID,
			&h.BetAmount,
			&h.WinAmount,
			&h.CashedOut,
			&h.CashoutMultiplier,
			&h.AutoCashout,
			&h.CreatedAt,
			&h.CashoutAt,
			&h.CrashPoint,
			&h.Hash,
			&h.Status,
		)
		if err != nil {
			log.Printf("❌ Row scan failed: %v", err)
			continue
		}

		history = append(history, gin.H{
			"game_id":            h.GameID,
			"bet_amount":         h.BetAmount,
			"win_amount":         h.WinAmount,
			"cashed_out":         h.CashedOut,
			"cashout_multiplier": h.CashoutMultiplier,
			"auto_cashout":       h.AutoCashout,
			"created_at":         h.CreatedAt,
			"cashout_at":         h.CashoutAt,
			"crash_point":        h.CrashPoint,
			"hash":               h.Hash,
			"status":             h.Status,
		})
	}

	log.Printf("✅ Found %d games in history for user %s", len(history), userID)
	c.JSON(200, history)
}

func (s *GameServer) endGame() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentGame == nil || s.currentGame.Status != "in_progress" {
		status := "nil"
		if s.currentGame != nil {
			status = s.currentGame.Status
		}
		log.Printf("❌ Cannot end game: game=%v, status=%v",
			s.currentGame != nil, status)
		return
	}

	log.Printf("🎮 Ending game %s at crash point %.2f",
		s.currentGame.GameID, s.currentGame.CrashPoint)

	s.currentGame.Status = "crashed"
	s.currentGame.EndTime = time.Now()

	// Create game history record with initialized Players slice
	history := &models.GameHistory{
		GameID:     s.currentGame.GameID,
		CrashPoint: s.currentGame.CrashPoint,
		Hash:       s.currentGame.Hash,
		StartTime:  s.currentGame.StartTime,
		EndTime:    s.currentGame.EndTime,
		Status:     "crashed",                                                   // Explicitly set status
		Players:    make([]models.PlayerHistory, 0, len(s.currentGame.Players)), // Pre-allocate slice
	}

	// Convert current players to player history
	for userID, player := range s.currentGame.Players {
		log.Printf("👤 Processing player %s: Bet=%.2f, Win=%.2f, CashedOut=%v",
			userID, player.BetAmount, player.WinAmount, player.CashedOut)

		history.Players = append(history.Players, models.PlayerHistory{
			UserID:      userID,
			BetAmount:   player.BetAmount,
			WinAmount:   player.WinAmount,
			CashedOut:   player.CashedOut,
			CashoutAt:   player.CashoutAt,
			AutoCashout: player.AutoCashout,
		})
	}

	// Save game history to database
	if err := s.db.SaveGameHistory(history); err != nil {
		log.Printf("❌ Failed to save game history: %v", err)
	} else {
		log.Printf("✅ Game history saved - ID: %s, Players: %d",
			history.GameID, len(history.Players))
	}

	// Start new game after delay
	time.AfterFunc(5*time.Second, s.startNewGame)
}
