package tests

import (
	"bytes"
	"crash-game/internal/database"
	"crash-game/internal/server"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

type GameHistoryTestServer struct {
	Server *server.GameServer
	DB     *database.Database
}

func TestGameHistoryTracking(t *testing.T) {
	// Setup server
	ts := SetupTestServer(t)
	defer ts.DB.Close()

	// Create test user and get token
	_, username := CreateTestUser(t, ts.DB)
	token := loginUser(t, username)
	t.Logf("Got auth token: %s", token)

	// Wait for new game to start betting phase
	WaitForGamePhaseHTTP(t, token, "betting")
	t.Log("New betting phase started")

	// Place bet
	placeBetHTTP(t, token, 100.0, nil)
	t.Log("Bet placed")

	// Wait for game to complete
	WaitForGamePhaseHTTP(t, token, "crashed")
	t.Log("Game ended")

	// Add a small delay to ensure game is saved
	time.Sleep(200 * time.Millisecond)

	// Get game history
	history := getGameHistoryHTTP(t, token)
	if len(history) == 0 {
		t.Error("Game history should not be empty")
	}
}

func TestPlayerHistory(t *testing.T) {
	ts := SetupTestServer(t)
	defer ts.DB.Close()

	// Create test user and get token
	userID, username := CreateTestUser(t, ts.DB)
	token := loginUser(t, username)

	// Get actual user ID from database after login
	user, err := ts.DB.GetUserByUsername(username)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	actualUserID := user.ID // This is the ID that will be used in bets
	t.Logf("Created test user - Username: %s, Original ID: %s, Actual ID: %s",
		username, userID, actualUserID)

	// Play through multiple games
	for i := 0; i < 3; i++ {
		// Wait for betting phase
		WaitForGamePhaseHTTP(t, token, "betting")
		t.Logf("Game %d: Starting betting phase", i+1)

		// Place bet
		placeBetHTTP(t, token, 100.0, nil)
		t.Logf("Game %d: Bet placed, amount: 100.0", i+1)

		// Wait for game to complete
		WaitForGamePhaseHTTP(t, token, "crashed")
		t.Logf("Game %d: Game completed", i+1)

		// Add a delay to ensure game is saved
		time.Sleep(1 * time.Second)

		// Debug: Check game state after completion
		req, _ := http.NewRequest("GET", "http://localhost:8080/api/game/current", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		client := &http.Client{}
		resp, _ := client.Do(req)
		body, _ := io.ReadAll(resp.Body)
		t.Logf("Game %d: Final game state: %s", i+1, string(body))

		// Verify game was saved
		currentHistory := getPlayerHistoryHTTP(t, token)
		t.Logf("Game %d: History count: %d", i+1, len(currentHistory))

		// Debug: Direct DB query after each game
		games, err := ts.DB.GetPlayerGameHistory(actualUserID)
		if err != nil {
			t.Logf("Game %d: DB query error: %v", i+1, err)
		} else {
			t.Logf("Game %d: DB records found: %d", i+1, len(games))
			for _, game := range games {
				t.Logf("Game %d: DB Record - Game: %s, Amount: %.2f, CrashPoint: %.2f",
					i+1, game.GameID, game.BetAmount, game.CrashPoint)
			}
		}
	}

	// Final check with longer delay
	time.Sleep(5 * time.Second)
	t.Log("Final check after delay")

	history := getPlayerHistoryHTTP(t, token)
	if len(history) != 3 {
		t.Errorf("Expected 3 games in player history, got %d", len(history))
		t.Logf("History: %+v", history)

		// Debug: Final DB state
		games, err := ts.DB.GetPlayerGameHistory(actualUserID)
		if err != nil {
			t.Logf("Final DB query error: %v", err)
		} else {
			t.Logf("Final DB query found %d games", len(games))
			for _, game := range games {
				t.Logf("Final DB Record - Game: %s, Amount: %.2f, CrashPoint: %.2f",
					game.GameID, game.BetAmount, game.CrashPoint)
			}
		}
	}
}

func getGameHistoryHTTP(t *testing.T, token string) []struct {
	GameID     string    `json:"gameId"`
	CrashPoint float64   `json:"crashPoint"`
	Hash       string    `json:"hash"`
	Seed       string    `json:"seed"`
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
} {
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/game/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to get game history: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Game history response: %s", string(body))

	var response struct {
		History []struct {
			GameID     string    `json:"gameId"`
			CrashPoint float64   `json:"crashPoint"`
			Hash       string    `json:"hash"`
			Seed       string    `json:"seed"`
			StartTime  time.Time `json:"startTime"`
			EndTime    time.Time `json:"endTime"`
		} `json:"history"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		t.Fatalf("Failed to decode game history: %v", err)
	}
	return response.History
}

func getPlayerHistoryHTTP(t *testing.T, token string) []struct {
	GameID     string    `json:"game_id"`
	BetAmount  float64   `json:"bet_amount"`
	WinAmount  float64   `json:"win_amount"`
	CrashPoint float64   `json:"crash_point"`
	CashedOut  bool      `json:"cashed_out"`
	CreatedAt  time.Time `json:"created_at"`
} {
	req, _ := http.NewRequest("GET", "http://localhost:8080/api/game/player/history", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to get player history: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	t.Logf("Player history response: %s", string(body))

	var history []struct {
		GameID     string    `json:"game_id"`
		BetAmount  float64   `json:"bet_amount"`
		WinAmount  float64   `json:"win_amount"`
		CrashPoint float64   `json:"crash_point"`
		CashedOut  bool      `json:"cashed_out"`
		CreatedAt  time.Time `json:"created_at"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&history); err != nil {
		t.Fatalf("Failed to decode player history: %v", err)
	}
	return history
}

func WaitForGamePhaseHTTP(t *testing.T, token string, phase string) {
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
