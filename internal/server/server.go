package server

import (
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
	CrashPoint float64            `json:"crashPoint"`
	Status     string             `json:"status"` // "waiting", "in_progress", "crashed"
	Players    map[string]*Player `json:"players"`
	Elapsed    float64            `json:"elapsed"`
	Hash       string             `json:"hash"`
	Saved      bool               `json:"-"`
}

type Player struct {
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

func (s *GameServer) gameLoop() {
	for {
		s.startNewGame()

		// Betting phase
		log.Printf("⏳ Betting phase started")
		time.Sleep(30 * time.Second)

		// Game phase
		s.mu.Lock()
		s.currentGame.Status = "in_progress"
		log.Printf("🎲 Game in progress - ID: %s", s.currentGame.GameID)
		s.mu.Unlock()

		// Wait until crash
		crashTime := time.Duration(math.Log(s.currentGame.CrashPoint) * 8 * float64(time.Second))
		time.Sleep(crashTime)

		// End game and save
		s.mu.Lock()
		s.currentGame.Status = "crashed"
		log.Printf("💥 Game crashed - ID: %s at %.2fx", s.currentGame.GameID, s.currentGame.CrashPoint)
		s.saveGameToHistory()
		s.mu.Unlock()

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

	// Start game progress after betting phase
	time.AfterFunc(5*time.Second, func() {
		go s.runGameProgress()
	})
}

func (s *GameServer) runGameProgress() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	s.mu.Lock()
	if s.currentGame == nil {
		s.mu.Unlock()
		return
	}
	s.currentGame.Status = "in_progress"
	s.mu.Unlock()

	log.Printf("🎮 Game %s is now in progress", s.currentGame.GameID)

	for {
		<-ticker.C

		s.mu.Lock()
		if s.currentGame == nil {
			s.mu.Unlock()
			return
		}

		elapsed := time.Since(s.currentGame.StartTime).Seconds()
		multiplier := math.Pow(math.E, 0.1*elapsed)
		s.currentGame.Elapsed = elapsed

		// Check if game should end
		if multiplier >= s.currentGame.CrashPoint {
			s.currentGame.Status = "crashed"
			s.saveGameToHistory()
			s.mu.Unlock()

			// Start new game after delay
			time.Sleep(2 * time.Second)
			s.startNewGame()
			return
		}
		s.mu.Unlock()
	}
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

	history := &models.GameHistory{
		GameID:     s.currentGame.GameID,
		CrashPoint: s.currentGame.CrashPoint,
		StartTime:  s.currentGame.StartTime,
		EndTime:    time.Now(),
		Hash:       s.currentGame.Hash,
		Players:    make([]models.PlayerHistory, 0),
	}

	if err := s.db.SaveGameHistory(history); err != nil {
		log.Printf("❌ SAVE: Failed to save game: %v", err)
		return
	}

	log.Printf("✅ SAVE: Game saved successfully")
}

func (s *GameServer) Run(addr string) error {
	// Initialize first game
	s.startNewGame()

	// Start the game loop in a goroutine
	go s.gameLoop()

	// Start the progress tracker in a goroutine
	go s.runGameProgress()

	// Log server startup
	log.Printf("Server starting on %s", addr)

	// Start the HTTP server
	return s.router.Run(addr)
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
			authenticated.POST("/game/verify", s.VerifyGameFairness)
			authenticated.POST("/user/withdraw", s.RequestWithdrawal)
		}
	}
}
