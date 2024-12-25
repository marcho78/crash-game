package tests

import (
	"crash-game/internal/game"
	"testing"
)

func TestGameVerification(t *testing.T) {
	// Generate a game result
	gameID := int64(1)
	result := game.GenerateNextGame(gameID)

	// Verify the game can be reproduced with the same hash
	h := game.VerifyGame(gameID, result.Hash)
	if !h {
		t.Error("Game verification failed")
	}

	// Verify the game cannot be verified with wrong hash
	h = game.VerifyGame(gameID, "wrong_hash")
	if h {
		t.Error("Game verification should fail with wrong hash")
	}
}

func TestGameFairness(t *testing.T) {
	crashPoints := make(map[float64]int)
	iterations := 10000

	for i := 0; i < iterations; i++ {
		result := game.GenerateNextGame(int64(i))
		crashPoints[result.CrashPoint]++

		// Verify crash points are within expected range
		if result.CrashPoint < 1.0 || result.CrashPoint > 10.0 {
			t.Errorf("Crash point %f outside valid range [1.0, 10.0]", result.CrashPoint)
		}
	}

	// Check distribution
	var total float64
	for point, count := range crashPoints {
		total += point * float64(count)
	}
	average := total / float64(iterations)

	// Expected average for uniform distribution between 1 and 10
	expectedAverage := 5.5
	tolerance := 0.5

	if average < expectedAverage-tolerance || average > expectedAverage+tolerance {
		t.Errorf("Unexpected average crash point: %f (expected around %f)", average, expectedAverage)
	}
}
