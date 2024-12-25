package tests

import (
	"crash-game/internal/database"
	"crash-game/internal/security"
	"crash-game/internal/server"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

type TestServer struct {
	Server *server.GameServer
	DB     *database.Database
}

func SetupTestServer(t *testing.T) *TestServer {
	db, err := database.NewDatabase("postgres://crashgamedb_user:March0s0ft@localhost:5432/crashgame?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	server := server.NewGameServer(db)

	// Start the game loop in a goroutine
	go server.StartGameLoop()

	// Wait for first game to initialize
	time.Sleep(100 * time.Millisecond)

	return &TestServer{
		Server: server,
		DB:     db,
	}
}

func TestConcurrentBetting(t *testing.T) {
	ts := SetupTestServer(t)
	defer ts.DB.Close()

	numUsers := 50
	var wg sync.WaitGroup
	errors := make(chan error, numUsers)
	userIDs := make([]string, 0, numUsers)
	var userIDsMutex sync.Mutex

	// Wait for betting phase
	WaitForGamePhase(t, ts.Server, "betting")

	// Create users and place bets concurrently
	for i := 0; i < numUsers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			userID, _ := CreateTestUser(t, ts.DB)
			err := ts.Server.PlaceBetForTest(userID, 100.0, nil)
			if err != nil {
				errors <- err
			} else {
				userIDsMutex.Lock()
				userIDs = append(userIDs, userID)
				userIDsMutex.Unlock()
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent betting error: %v", err)
	}

	// Verify game state
	game := ts.Server.CurrentGame()
	if game == nil {
		t.Fatal("Game should not be nil")
	}

	if len(game.Players) != numUsers {
		t.Errorf("Expected %d players in game, got %d", numUsers, len(game.Players))
	}

	// Verify each user's bet was recorded
	for _, userID := range userIDs {
		player, exists := game.Players[userID]
		if !exists {
			t.Errorf("Player %s not found in game", userID)
			continue
		}
		if player.BetAmount != 100.0 {
			t.Errorf("Expected bet amount 100.0 for player %s, got %f", userID, player.BetAmount)
		}
	}
}

func TestConcurrentCashouts(t *testing.T) {
	ts := SetupTestServer(t)
	defer ts.DB.Close()

	numUsers := 20
	userIDs := make([]string, numUsers)

	// Create users and place bets
	for i := 0; i < numUsers; i++ {
		userID, _ := CreateTestUser(t, ts.DB)
		userIDs[i] = userID
	}

	// Wait for betting phase and place bets
	WaitForGamePhase(t, ts.Server, "betting")
	for _, userID := range userIDs {
		err := ts.Server.PlaceBetForTest(userID, 100.0, nil)
		if err != nil {
			t.Fatalf("Failed to place bet: %v", err)
		}
	}

	// Wait for game to start
	WaitForGamePhase(t, ts.Server, "in_progress")

	// Attempt concurrent cashouts
	var wg sync.WaitGroup
	errors := make(chan error, numUsers)

	for _, userID := range userIDs {
		wg.Add(1)
		go func(uid string) {
			defer wg.Done()
			_, err := ts.Server.CashoutForTest(uid)
			if err != nil {
				errors <- err
			}
		}(userID)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent cashout error: %v", err)
	}
}

func CreateTestUser(t *testing.T, db *database.Database) (string, string) {
	userID := uuid.New().String()
	username := fmt.Sprintf("testuser_%s", userID[:8])

	hashedPass, _ := security.HashPassword("testpass123")
	err := db.CreateUser(username, hashedPass)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return userID, username
}

func WaitForGamePhase(t *testing.T, server *server.GameServer, phase string) {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for game phase: %s", phase)
		case <-ticker.C:
			currentGame := server.CurrentGame()
			if currentGame != nil && currentGame.Status == phase {
				return
			}
		}
	}
}
