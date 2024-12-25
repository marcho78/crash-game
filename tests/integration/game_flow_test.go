package tests

import (
	"bytes"
	"crash-game/internal/database"
	"crash-game/internal/security"
	"crash-game/internal/server"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

type TestGameServer struct {
	Server *server.GameServer
	DB     *database.Database
}

func SetupGameFlowTestServer(t *testing.T) *TestGameServer {
	// Use the same database as main application
	db, err := database.NewDatabase("postgres://crashgamedb_user:March0s0ft@localhost:5432/crashgame?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Connect to the existing server running on :8080
	server := server.ConnectToServer("http://localhost:8080", db)

	return &TestGameServer{
		Server: server,
		DB:     db,
	}
}

func WaitForGamePhaseHTTP(t *testing.T, server *server.GameServer, phase string) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	client := &http.Client{}

	// Login first to get token
	loginResp, err := client.Post("http://localhost:8080/api/auth/login",
		"application/json",
		strings.NewReader(`{"username":"testuser_691a9781","password":"testpass123"}`))
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	defer loginResp.Body.Close()

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&authResp); err != nil {
		t.Fatalf("Failed to decode auth response: %v", err)
	}

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for game phase: %s", phase)
		case <-ticker.C:
			req, err := http.NewRequest("GET", "http://localhost:8080/api/game/current", nil)
			if err != nil {
				continue
			}
			// Add auth token
			req.Header.Add("Authorization", "Bearer "+authResp.Token)

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			defer resp.Body.Close()

			var game struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&game); err != nil {
				continue
			}

			if game.Status == phase {
				return
			}
		}
	}
}

func TestCompleteGameFlow(t *testing.T) {
	ts := SetupGameFlowTestServer(t)
	defer ts.DB.Close()

	// Create test users and store their IDs from the database
	user1Name := fmt.Sprintf("testuser_%s", uuid.New().String()[:8])
	user2Name := fmt.Sprintf("testuser_%s", uuid.New().String()[:8])

	hashedPass, _ := security.HashPassword("testpass123")
	err := ts.DB.CreateUser(user1Name, hashedPass)
	if err != nil {
		t.Fatalf("Failed to create test user 1: %v", err)
	}
	err = ts.DB.CreateUser(user2Name, hashedPass)
	if err != nil {
		t.Fatalf("Failed to create test user 2: %v", err)
	}

	// Login both users
	token1 := loginUser(t, user1Name)
	token2 := loginUser(t, user2Name)

	// Get user IDs from the database after login
	user1, err := ts.DB.GetUserByUsername(user1Name)
	if err != nil {
		t.Fatalf("Failed to get user1: %v", err)
	}
	user2, err := ts.DB.GetUserByUsername(user2Name)
	if err != nil {
		t.Fatalf("Failed to get user2: %v", err)
	}
	user1ID := user1.ID
	user2ID := user2.ID

	// Wait for betting phase
	WaitForGamePhaseHTTP(t, ts.Server, "betting")

	// Place bets via HTTP
	placeBetHTTP(t, token1, 100.0, nil)
	auto := 2.0
	placeBetHTTP(t, token2, 150.0, &auto)

	t.Logf("DEBUG: Placed bets - User1: manual, User2: auto=%.2fx", auto)

	// Wait for game to start
	WaitForGamePhaseHTTP(t, ts.Server, "in_progress")

	t.Log("DEBUG: Game is now in progress")

	// Manual cashout for user 1 after a delay
	time.Sleep(2 * time.Second)
	multiplier := cashoutHTTP(t, token1)
	t.Logf("DEBUG: Manual cashout executed for user 1 at %.2fx", multiplier)

	// Get game state before crash with retries
	var gameBeforeCrash *struct {
		Status  string `json:"status"`
		Players map[string]struct {
			UserID    string     `json:"userId"`
			BetAmount float64    `json:"betAmount"`
			CashedOut bool       `json:"cashedOut"`
			CashoutAt *time.Time `json:"cashoutAt"`
			WinAmount float64    `json:"winAmount"`
		} `json:"players"`
	}

	t.Log("DEBUG: Getting game state before crash...")

	// Retry up to 10 times with a 1-second delay
	for i := 0; i < 10; i++ {
		gameBeforeCrash = getGameStateHTTP(t, token1)
		t.Logf("DEBUG: Attempt %d - Players: %+v", i+1, gameBeforeCrash.Players)
		if len(gameBeforeCrash.Players) > 0 {
			// Check if player2 has cashed out and multiplier is >= 2.0
			if player2, exists := gameBeforeCrash.Players[user2ID]; exists {
				t.Logf("DEBUG: Player2 state - CashedOut: %v, AutoCashout: %v", player2.CashedOut, auto)
				if player2.CashedOut && player2.WinAmount > 0 {
					break
				}
			}
		}
		time.Sleep(1 * time.Second)
	}

	t.Logf("Game state before crash: %+v", gameBeforeCrash)

	// Wait for game to end
	WaitForGamePhaseHTTP(t, ts.Server, "crashed")

	// Verify results using the state we captured before crash
	player1, exists := gameBeforeCrash.Players[user1ID]
	if !exists {
		t.Fatalf("User1 not found in game players")
	}
	if !player1.CashedOut {
		t.Error("User 1 should be marked as cashed out")
	}
	if player1.WinAmount <= player1.BetAmount {
		t.Error("User 1 should have won more than bet amount")
	}

	player2, exists := gameBeforeCrash.Players[user2ID]
	if !exists {
		t.Fatalf("User2 not found in game players")
	}
	if !player2.CashedOut {
		t.Error("User 2 should be marked as cashed out (auto)")
	}
}

func loginUser(t *testing.T, username string) string {
	loginResp, err := http.Post("http://localhost:8080/api/auth/login",
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"username":"%s","password":"testpass123"}`, username)))
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}
	defer loginResp.Body.Close()

	var authResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&authResp); err != nil {
		t.Fatalf("Failed to decode auth response: %v", err)
	}
	return authResp.Token
}

func placeBetHTTP(t *testing.T, token string, amount float64, autoCashout *float64) {
	betData := struct {
		Amount      float64  `json:"amount"`
		AutoCashout *float64 `json:"auto_cashout,omitempty"`
	}{
		Amount:      amount,
		AutoCashout: autoCashout,
	}

	betJSON, _ := json.Marshal(betData)
	req, _ := http.NewRequest("POST", "http://localhost:8080/api/bet", bytes.NewBuffer(betJSON))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to place bet: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Failed to place bet, status: %d", resp.StatusCode)
	}
}

func cashoutHTTP(t *testing.T, token string) float64 {
	req, _ := http.NewRequest("POST", "http://localhost:8080/api/cashout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send cashout request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Failed to cashout, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Multiplier float64 `json:"multiplier"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode cashout response: %v", err)
	}
	return result.Multiplier
}

func getGameStateHTTP(t *testing.T, token string) *struct {
	Status  string `json:"status"`
	Players map[string]struct {
		UserID    string     `json:"userId"`
		BetAmount float64    `json:"betAmount"`
		CashedOut bool       `json:"cashedOut"`
		CashoutAt *time.Time `json:"cashoutAt"`
		WinAmount float64    `json:"winAmount"`
	} `json:"players"`
} {
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/game/current", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to get current game: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Raw game state response: %s", string(body))

	var game struct {
		Status  string `json:"status"`
		Players map[string]struct {
			UserID    string     `json:"userId"`
			BetAmount float64    `json:"betAmount"`
			CashedOut bool       `json:"cashedOut"`
			CashoutAt *time.Time `json:"cashoutAt"`
			WinAmount float64    `json:"winAmount"`
		} `json:"players"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&game); err != nil {
		t.Fatalf("Failed to decode game state: %v", err)
	}
	return &game
}
