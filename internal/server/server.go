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
		// Start new game
		s.mu.Lock()
		s.startNewGame()
		s.currentGame.Status = "betting"
		log.Printf("Game %s started - Betting phase", s.currentGame.GameID)
		s.mu.Unlock()

		// Betting phase (5 seconds)
		time.Sleep(5 * time.Second)

		// Start game phase
		s.mu.Lock()
		s.currentGame.Status = "in_progress"
		s.currentGame.StartTime = time.Now()
		log.Printf("Game %s in progress - Crash point: %.2f", s.currentGame.GameID, s.currentGame.CrashPoint)
		s.mu.Unlock()

		// Wait until crash point
		crashTime := time.Duration(float64(time.Second) * s.currentGame.CrashPoint)

		time.Sleep(crashTime)

		// End game
		s.mu.Lock()
		s.currentGame.Status = "crashed"
		log.Printf("Game %s crashed at %.2fx", s.currentGame.GameID, s.currentGame.CrashPoint)
		s.saveGameToHistory()
		s.mu.Unlock()

		// Short delay before next game
		time.Sleep(2 * time.Second)
	}
}

func (s *GameServer) startNewGame() {
	gameID := uuid.New().String()
	crashPoint := 2.0 // For testing, you can make this random later

	s.currentGame = &GameState{
		GameID:     gameID,
		StartTime:  time.Now(),
		CrashPoint: crashPoint,
		Status:     "betting",
		Players:    make(map[string]*Player),
		Hash:       fmt.Sprintf("%x", sha256.Sum256([]byte(gameID))),
	}
}

func (s *GameServer) runGameProgress() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

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

func generateHash() string {
	data := make([]byte, 32)
	rand.Read(data)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func generateCrashPoint() float64 {
	return 1.0 + rand.Float64()*9.0
}

// Initialize random seed in init
func init() {
	rand.Seed(time.Now().UnixNano())
}

func (s *GameServer) saveGameToHistory() {
	gameHistory := models.GameHistory{
		GameID:     s.currentGame.GameID,
		CrashPoint: s.currentGame.CrashPoint,
		StartTime:  s.currentGame.StartTime,
		EndTime:    time.Now(),
		Hash:       s.currentGame.Hash,
		Players:    make([]models.PlayerHistory, 0),
	}

	for userID, player := range s.currentGame.Players {
		playerHistory := models.PlayerHistory{
			UserID:     userID,
			BetAmount:  player.BetAmount,
			CashedOut:  player.CashedOut,
			CashoutAt:  player.CashoutAt,
			WinAmount:  player.WinAmount,
			Multiplier: player.WinAmount / player.BetAmount,
		}
		gameHistory.Players = append(gameHistory.Players, playerHistory)
	}

	s.historyMu.Lock()
	s.gameHistory = append(s.gameHistory, gameHistory)
	s.historyMu.Unlock()
}

func (s *GameServer) Run(addr string) error {
	// Initialize first game
	s.startNewGame()

	// Start the game loop in a goroutine
	go s.gameLoop()

	// Log server startup
	log.Printf("Server starting on %s", addr)

	// Start the HTTP server and block until it returns
	if err := s.router.Run(addr); err != nil {
		log.Printf("Server failed to start: %v", err)
		return err
	}

	return nil
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
		protected := api.Group("")
		protected.Use(AuthMiddleware())
		{
			protected.GET("/user/balance", s.GetBalance)
			protected.POST("/bet", s.PlaceBet)
			protected.POST("/cashout", s.Cashout)
			protected.GET("/game/current", s.GetCurrentGame)
		}
	}
}
