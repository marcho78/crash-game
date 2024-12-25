package tests

import (
	"testing"
	"time"
)

func TestGameStateTransitions(t *testing.T) {
	ts := SetupTestServer(t)
	defer ts.DB.Close()

	// Wait for betting phase
	WaitForGamePhase(t, ts.Server, "betting")

	// Place a test bet
	userID, _ := CreateTestUser(t, ts.DB)
	err := ts.Server.PlaceBetForTest(userID, 100.0, nil)
	if err != nil {
		t.Fatalf("Failed to place bet: %v", err)
	}

	// Wait for game to start
	WaitForGamePhase(t, ts.Server, "in_progress")

	// Verify game state
	game := ts.Server.CurrentGame()
	if game == nil {
		t.Fatal("Game should not be nil")
	}

	if len(game.Players) != 1 {
		t.Errorf("Expected 1 player, got %d", len(game.Players))
	}

	// Wait for game to end
	WaitForGamePhase(t, ts.Server, "crashed")
}

func TestGameTimings(t *testing.T) {
	ts := SetupTestServer(t)
	defer ts.DB.Close()

	// Wait for new game
	WaitForGamePhase(t, ts.Server, "betting")
	start := time.Now()

	// Wait for betting phase to end
	WaitForGamePhase(t, ts.Server, "in_progress")
	bettingDuration := time.Since(start)

	if bettingDuration < 4*time.Second || bettingDuration > 6*time.Second {
		t.Errorf("Unexpected betting phase duration: %v", bettingDuration)
	}
}
