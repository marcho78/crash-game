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

type CashoutTestServer struct {
	Server *server.GameServer
	DB     *database.Database
}

func SetupCashoutTestServer(t *testing.T) *CashoutTestServer {
	db, err := database.NewDatabase("postgres://crashgamedb_user:March0s0ft@localhost:5432/crashgame?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	server := server.NewGameServer(db)
	return &CashoutTestServer{Server: server, DB: db}
}

func CreateCashoutTestUser(t *testing.T, db *database.Database) (string, string) {
	username := fmt.Sprintf("testuser_%s", uuid.New().String()[:8])
	hashedPass, _ := security.HashPassword("testpass123")
	err := db.CreateUser(username, hashedPass)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}
	return username, username
}

func loginCashoutUser(t *testing.T, username string) string {
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

func WaitForCashoutGamePhase(t *testing.T, token string, phase string) {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for game phase: %s", phase)
		case <-ticker.C:
			req, _ := http.NewRequest("GET", "http://localhost:8080/api/game/current", nil)
			req.Header.Set("Authorization", "Bearer "+token)

			client := &http.Client{}
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

func placeCashoutBetHTTP(t *testing.T, token string, amount float64, autoCashout *float64) {
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

func TestCashout(t *testing.T) {
	ts := SetupCashoutTestServer(t)
	defer ts.DB.Close()

	// Create test user and get token
	_, username := CreateCashoutTestUser(t, ts.DB)
	token := loginCashoutUser(t, username)

	// Wait for betting phase
	WaitForCashoutGamePhase(t, token, "betting")

	// Place bet via HTTP
	placeCashoutBetHTTP(t, token, 100.0, nil)

	// Wait for game to start
	WaitForCashoutGamePhase(t, token, "in_progress")

	// Wait a bit for multiplier to increase
	time.Sleep(2 * time.Second)

	// Try to cash out
	multiplier := cashoutHTTP(t, token)
	if multiplier <= 1.0 {
		t.Errorf("Invalid cashout multiplier: %f", multiplier)
	}
}

func TestAutoCashout(t *testing.T) {
	ts := SetupCashoutTestServer(t)
	defer ts.DB.Close()

	// Create test user and get token
	_, username := CreateCashoutTestUser(t, ts.DB)
	token := loginCashoutUser(t, username)

	// Wait for betting phase
	WaitForCashoutGamePhase(t, token, "betting")

	// Place bet with auto-cashout
	auto := 2.0
	placeCashoutBetHTTP(t, token, 100.0, &auto)

	// Wait for game to start
	WaitForCashoutGamePhase(t, token, "in_progress")

	// Wait and check multiple times for auto-cashout
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		game := getGameStateHTTP(t, token)
		for _, player := range game.Players {
			if player.CashedOut && player.WinAmount > 0 {
				return // Test passed
			}
		}
	}
	t.Error("Player should have been automatically cashed out")
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
