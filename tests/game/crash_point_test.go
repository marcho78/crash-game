package tests

import (
	"crash-game/internal/game"
	"testing"
)

func TestCrashPointCalculation(t *testing.T) {
	tests := []struct {
		name     string
		gameID   int64
		expected float64
	}{
		{
			name:     "Game ID 1",
			gameID:   1,
			expected: 1.0, // minimum crash point
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := game.GenerateNextGame(tt.gameID)
			if result.CrashPoint < 1.0 {
				t.Errorf("Crash point %f is less than minimum 1.0", result.CrashPoint)
			}
		})
	}
}

func TestCrashPointDistribution(t *testing.T) {
	var sum float64
	iterations := 1000

	for i := 0; i < iterations; i++ {
		result := game.GenerateNextGame(int64(i))
		sum += result.CrashPoint

		if result.CrashPoint < 1.0 {
			t.Errorf("Invalid crash point %f (less than 1.0)", result.CrashPoint)
		}
		if result.CrashPoint > 10.0 {
			t.Errorf("Invalid crash point %f (greater than 10.0)", result.CrashPoint)
		}
	}

	average := sum / float64(iterations)
	expectedAverage := 5.5 // Average of uniform distribution between 1 and 10
	tolerance := 0.5

	if average < expectedAverage-tolerance || average > expectedAverage+tolerance {
		t.Errorf("Unexpected average crash point: %f (expected around %f)", average, expectedAverage)
	}
}

func TestGameHashGeneration(t *testing.T) {
	game1 := game.GenerateNextGame(1)
	game2 := game.GenerateNextGame(1)

	if game1.Hash != game2.Hash {
		t.Error("Same game ID should produce same hash")
	}

	game3 := game.GenerateNextGame(2)
	if game1.Hash == game3.Hash {
		t.Error("Different game IDs should produce different hashes")
	}
}
