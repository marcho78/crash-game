package tests

import (
	"crash-game/internal/database"
	"crash-game/internal/security"
	"crash-game/internal/server"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

type TestGameServer struct {
	Server *server.GameServer
	DB     *database.Database
}

func SetupTestServer(t *testing.T) *TestGameServer {
	db, err := database.NewDatabase("postgres://crashgamedb_user:March0s0ft@localhost:5432/crashgame?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	server := server.NewGameServer(db)

	// Start the game loop in a goroutine
	go server.StartGameLoop()

	// Wait for first game to initialize
	time.Sleep(100 * time.Millisecond)

	return &TestGameServer{
		Server: server,
		DB:     db,
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

	// Get the actual user ID from the database
	user, err := db.GetUserByUsername(username)
	if err != nil {
		t.Fatalf("Failed to get created user: %v", err)
	}

	return user.ID, username
}

func WaitForGamePhase(t *testing.T, server *server.GameServer, phase string) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for game phase: %s", phase)
		case <-ticker.C:
			game := server.CurrentGame()
			if game != nil && game.Status == phase {
				time.Sleep(100 * time.Millisecond)
				return
			}
		}
	}
}
