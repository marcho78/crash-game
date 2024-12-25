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

	return &TestServer{
		Server: server,
		DB:     db,
	}
}

func TestBetHandling(t *testing.T) {
	ts := SetupTestServer(t)
	defer ts.DB.Close()

	tests := []struct {
		name         string
		amount       float64
		autoCashout  *float64
		expectedCode int
	}{
		{"valid bet", 100.0, nil, http.StatusOK},
		{"zero bet", 0.0, nil, http.StatusBadRequest},
		{"negative bet", -50.0, nil, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test user and get token
			_, username := CreateTestUser(t, ts.DB)
			token := loginUser(t, username)

			// Wait for betting phase
			WaitForGamePhase(t, token, "betting")

			// Place bet via HTTP
			betData := struct {
				Amount      float64  `json:"amount"`
				AutoCashout *float64 `json:"auto_cashout,omitempty"`
			}{
				Amount:      tt.amount,
				AutoCashout: tt.autoCashout,
			}

			betJSON, _ := json.Marshal(betData)
			req, _ := http.NewRequest("POST", "http://localhost:8080/api/bet", bytes.NewBuffer(betJSON))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to send bet request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedCode {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("Expected status code %d, got %d. Response: %s",
					tt.expectedCode, resp.StatusCode, string(body))
			}
		})
	}
}

func TestDoubleBetPrevention(t *testing.T) {
	ts := SetupTestServer(t)
	defer ts.DB.Close()

	// Create test user and get token
	_, username := CreateTestUser(t, ts.DB)
	token := loginUser(t, username)

	// Wait for betting phase
	WaitForGamePhase(t, token, "betting")

	// Place first bet
	placeBetHTTP(t, token, 100.0, nil)

	// Attempt second bet (should fail)
	betData := struct {
		Amount float64 `json:"amount"`
	}{
		Amount: 50.0,
	}

	betJSON, _ := json.Marshal(betData)
	req, _ := http.NewRequest("POST", "http://localhost:8080/api/bet", bytes.NewBuffer(betJSON))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send bet request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code %d for double bet, got %d",
			http.StatusBadRequest, resp.StatusCode)
	}
}

func WaitForGamePhase(t *testing.T, token string, phase string) {
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
				t.Logf("Error getting game state: %v", err)
				continue
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			var game struct {
				Status string `json:"status"`
			}
			if err := json.NewDecoder(bytes.NewReader(body)).Decode(&game); err != nil {
				t.Logf("Error decoding response: %v. Body: %s", err, string(body))
				continue
			}

			t.Logf("Current game phase: %s, waiting for: %s", game.Status, phase)
			if game.Status == phase {
				return
			}
		}
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
