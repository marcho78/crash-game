package server

import (
	"errors"
	"math"
	"math/rand"
	"sync"
	"time"

	"crash-game/internal/database"
	"crash-game/internal/models"
	"crash-game/internal/notification"
	"crash-game/internal/security"

	"crypto/sha256"
	"fmt"

	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type GameState struct {
	GameID     string             `json:"gameId"`
	StartTime  time.Time          `json:"startTime"`
	CrashPoint float64            `json:"-"`
	Status     string             `json:"status"` // "waiting", "in_progress", "crashed"
	Players    map[string]*Player `json:"players"`
	Elapsed    float64            `json:"elapsed"`
	Hash       string             `json:"hash"`
	Saved      bool               `json:"-"`
	EndTime    time.Time          `json:"endTime"`
}

type Player struct {
	UserID      string     `json:"userId"`
	BetAmount   float64    `json:"betAmount"`
	CashedOut   bool       `json:"cashedOut"`
	CashoutAt   *time.Time `json:"cashoutAt,omitempty"`
	WinAmount   float64    `json:"winAmount"`
	AutoCashout *float64   `json:"autoCashout,omitempty"`
}

type GameHistory struct {
	GameID     string          `json:"gameId"`
	CrashPoint float64         `json:"crashPoint"`
	StartTime  time.Time       `json:"startTime"`
	EndTime    time.Time       `json:"endTime"`
	Hash       string          `json:"hash"`
	Players    []PlayerHistory `json:"players"`
}

type PlayerHistory struct {
	UserID     string     `json:"userId"`
	BetAmount  float64    `json:"betAmount"`
	CashedOut  bool       `json:"cashedOut"`
	CashoutAt  *time.Time `json:"cashoutAt,omitempty"`
	WinAmount  float64    `json:"winAmount"`
	Multiplier float64    `json:"multiplier,omitempty"`
}

type GameServer struct {
	router              *gin.Engine
	currentGame         *GameState
	mu                  sync.RWMutex
	db                  *database.Database
	historyMu           sync.RWMutex
	gameHistory         []models.GameHistory
	notificationManager *notification.NotificationManager
	csrfManager         *security.CSRFManager
	clients             sync.Map
	baseURL             string
}

func NewGameServer(db *database.Database) *GameServer {
	router := gin.Default()

	server := &GameServer{
		db:          db,
		router:      router,
		gameHistory: make([]models.GameHistory, 0),
		clients:     sync.Map{},
		currentGame: &GameState{
			Status:  "betting",
			Players: make(map[string]*Player),
		},
	}

	// Setup routes
	server.setupRoutes()

	return server
}

func (s *GameServer) Run(addr string) error {
	// Initialize first game
	s.startNewGame()

	// Start the game loop in a goroutine
	go s.gameLoop()

	// Log server startup
	log.Printf("Server starting on %s", addr)

	// Start the HTTP server
	return s.router.Run(addr)
}

func (s *GameServer) gameLoop() {
	for {
		// Start new game
		s.startNewGame()

		// Betting phase (5 seconds)
		log.Printf("⏳ Betting phase started")
		time.Sleep(5 * time.Second)

		// Game phase
		s.mu.Lock()
		if s.currentGame != nil {
			s.currentGame.Status = "in_progress"
			gameID := s.currentGame.GameID
			crashPoint := s.currentGame.CrashPoint
			log.Printf("🎮 DEBUG: [GAME] Starting game %s", s.currentGame.GameID)

			// Log all players and their auto-cashouts at game start
			for userID, player := range s.currentGame.Players {
				autoCashoutValue := "<nil>"
				if player.AutoCashout != nil {
					autoCashoutValue = fmt.Sprintf("%.2f", *player.AutoCashout)
				}
				log.Printf("👤 DEBUG: [GAME] Player %s starting with AutoCashout: %s",
					userID, autoCashoutValue)
			}
			s.mu.Unlock()

			// Wait until crash, checking auto-cashouts periodically
			start := time.Now()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for {
				elapsed := time.Since(start).Seconds()
				multiplier := math.Pow(math.E, 0.1*elapsed)

				log.Printf("🎲 DEBUG: [GAME] Current multiplier: %.2fx", multiplier)

				// Check if we should crash
				if multiplier >= crashPoint {
					log.Printf("💥 DEBUG: [GAME] Crashing at %.2fx", multiplier)
					break
				}

				// Check auto-cashouts
				s.mu.Lock()
				for userID, player := range s.currentGame.Players {
					autoCashoutValue := "<nil>"
					if player.AutoCashout != nil {
						autoCashoutValue = fmt.Sprintf("%.2f", *player.AutoCashout)
					}
					log.Printf("👤 DEBUG: [AUTO] Player %s check - AutoCashout: %s, CashedOut: %v, Multiplier: %.2fx, Pointer: %p",
						userID, autoCashoutValue, player.CashedOut, multiplier, player.AutoCashout)

					if !player.CashedOut && player.AutoCashout != nil {
						targetMultiplier := *player.AutoCashout
						log.Printf("🎯 DEBUG: [AUTO] Comparing %.2f >= %.2f for user %s",
							multiplier, targetMultiplier, userID)

						if multiplier >= targetMultiplier {
							log.Printf("💰 DEBUG: [AUTO] TRIGGER - User %s at %.2fx (target: %.2fx)",
								userID, multiplier, *player.AutoCashout)
							player.CashedOut = true
							player.WinAmount = player.BetAmount * multiplier
							now := time.Now()
							player.CashoutAt = &now

							// Credit winnings
							if err := s.db.UpdateBalance(userID, player.WinAmount, "credit"); err != nil {
								log.Printf("❌ DEBUG: [AUTO] Failed to credit auto-cashout: %v", err)
							} else {
								log.Printf("✅ DEBUG: [AUTO] Success - User: %s, Amount: %.2f at %.2fx",
									userID, player.WinAmount, multiplier)
							}
						}
					}
				}
				s.mu.Unlock()

				<-ticker.C
			}

			// End game and save
			s.mu.Lock()
			s.currentGame.Status = "crashed"
			log.Printf("💥 Game crashed - ID: %s at %.2fx", gameID, crashPoint)
			s.saveGameToHistory()
			s.mu.Unlock()
		} else {
			s.mu.Unlock()
		}

		// Short delay between games
		time.Sleep(2 * time.Second)
	}
}

func (s *GameServer) startNewGame() {
	gameID := uuid.New().String()
	hash := generateHash(gameID)
	crashPoint := generateCrashPoint()

	s.mu.Lock()
	s.currentGame = &GameState{
		GameID:     gameID,
		StartTime:  time.Now().Add(5 * time.Second),
		CrashPoint: crashPoint,
		Status:     "betting",
		Players:    make(map[string]*Player),
		Hash:       hash,
	}
	s.mu.Unlock()

	log.Printf("🎮 NEW GAME - ID: %s", gameID)
	log.Printf("🎲 Game details - Hash: %s, CrashPoint: %.2f", hash, crashPoint)
}

func generateHash(gameID string) string {
	data := []byte(gameID)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func generateCrashPoint() float64 {
	return 1.0 + rand.Float64()*9.0
}

// Initialize random seed in init
func init() {
	rand.Seed(time.Now().UnixNano())
}

func (s *GameServer) saveGameToHistory() {
	if s.currentGame == nil {
		return
	}

	// Add a saved flag check
	if s.currentGame.Saved {
		log.Printf("Game %s already saved, skipping", s.currentGame.GameID)
		return
	}

	log.Printf("💾 SAVE: Game %s - Hash: %s", s.currentGame.GameID, s.currentGame.Hash)

	// Mark as saved before doing the actual save
	s.currentGame.Saved = true

	// Create game history with players
	history := &models.GameHistory{
		GameID:     s.currentGame.GameID,
		CrashPoint: s.currentGame.CrashPoint,
		StartTime:  s.currentGame.StartTime,
		EndTime:    time.Now(),
		Hash:       s.currentGame.Hash,
		Status:     "crashed",
		Players:    make([]models.PlayerHistory, 0, len(s.currentGame.Players)),
	}

	// Add players to history
	for userID, player := range s.currentGame.Players {
		history.Players = append(history.Players, models.PlayerHistory{
			UserID:      userID,
			BetAmount:   player.BetAmount,
			WinAmount:   player.WinAmount,
			CashedOut:   player.CashedOut,
			CashoutAt:   player.CashoutAt,
			AutoCashout: player.AutoCashout,
		})
	}

	if err := s.db.SaveGameHistory(history); err != nil {
		log.Printf("❌ SAVE: Failed to save game: %v", err)
		return
	}

	log.Printf("✅ SAVE: Game saved successfully with %d players", len(history.Players))
}

func (s *GameServer) setupRoutes() {
	// Health check
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API routes
	api := s.router.Group("/api")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", s.Register)
			auth.POST("/login", s.Login)
		}

		// Protected routes
		authenticated := api.Group("")
		authenticated.Use(AuthMiddleware())
		{
			authenticated.GET("/user/balance", s.GetBalance)
			authenticated.POST("/bet", s.PlaceBet)
			authenticated.POST("/cashout", s.Cashout)
			authenticated.GET("/game/current", s.GetCurrentGame)
			authenticated.GET("/game/history", s.GetGameHistory)
			authenticated.GET("/game/player/history", s.GetPlayerGameHistory)
			authenticated.POST("/game/verify", s.VerifyGameFairness)
			authenticated.POST("/user/withdraw", s.RequestWithdrawal)
		}
	}
}

func (s *GameServer) CurrentGame() *GameState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentGame
}

func (s *GameServer) PlaceBetForTest(userID string, amount float64, autoCashout *float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentGame == nil || s.currentGame.Status != "betting" {
		return errors.New("game not accepting bets")
	}

	s.currentGame.Players[userID] = &Player{
		UserID:      userID,
		BetAmount:   amount,
		AutoCashout: autoCashout,
	}

	return nil
}

func (s *GameServer) CashoutForTest(userID string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.currentGame == nil || s.currentGame.Status != "in_progress" {
		return 0, errors.New("game not in progress")
	}

	player, exists := s.currentGame.Players[userID]
	if !exists {
		return 0, errors.New("no bet found for this game")
	}

	if player.CashedOut {
		return 0, errors.New("already cashed out")
	}

	multiplier := s.currentGame.GetCurrentMultiplier()
	player.CashedOut = true
	player.WinAmount = player.BetAmount * multiplier

	return multiplier, nil
}

func (s *GameServer) GetGameHistoryInMemory(limit int) ([]models.GameHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit > len(s.gameHistory) {
		limit = len(s.gameHistory)
	}

	history := make([]models.GameHistory, limit)
	copy(history, s.gameHistory[:limit])
	return history, nil
}

func (s *GameServer) GetGameHistoryForTest(limit int) ([]models.GameHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit > len(s.gameHistory) {
		limit = len(s.gameHistory)
	}

	history := make([]models.GameHistory, limit)
	copy(history, s.gameHistory[:limit])
	return history, nil
}

// PlaceBetDirect is for testing - allows direct bet placement without gin context
func (s *GameServer) PlaceBetDirect(userID string, amount float64, autoCashout *float64) error {
	if s.currentGame == nil || s.currentGame.Status != "betting" {
		return errors.New("game not accepting bets")
	}

	balance, err := s.db.GetUserBalance(userID)
	if err != nil {
		return err
	}

	if balance < amount {
		return errors.New("insufficient balance")
	}

	if amount > 1000.0 {
		return errors.New("invalid amount: maximum bet is 1000.00")
	}

	if err := s.db.UpdateBalance(userID, amount, "debit"); err != nil {
		return err
	}

	s.currentGame.Players[userID] = &Player{
		BetAmount:   amount,
		CashedOut:   false,
		WinAmount:   0,
		AutoCashout: autoCashout,
	}

	return nil
}

func (s *GameServer) GetPlayerHistory(userID string, limit int) ([]models.GameHistory, error) {
	return s.db.GetGameHistory(userID)
}

func (s *GameServer) GetRecentGames(limit int) ([]models.GameHistory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit > len(s.gameHistory) {
		limit = len(s.gameHistory)
	}

	history := make([]models.GameHistory, limit)
	copy(history, s.gameHistory[:limit])
	return history, nil
}

func (s *GameServer) StartGameLoop() {
	// Initialize first game
	s.startNewGame()

	// Start the game loop
	s.gameLoop()
}

// ConnectToServer creates a client connection to an existing server
func ConnectToServer(baseURL string, db *database.Database) *GameServer {
	return &GameServer{
		db:      db,
		router:  gin.Default(),
		baseURL: baseURL,
	}
}

func (s *GameServer) GetCurrentGameState() (*GameState, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentGame, nil
}
