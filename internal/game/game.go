package game

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"strconv"
)

type GameResult struct {
	GameID     int64   `json:"gameId"`
	CrashPoint float64 `json:"crashPoint"`
	Hash       string  `json:"hash"`
	Seed       string  `json:"seed"`
}

// ServerSeed - in production, this should be stored securely
const ServerSeed = "your-server-seed"

func GenerateNextGame(gameId int64) GameResult {
	// Create game hash using HMAC-SHA256
	h := hmac.New(sha256.New, []byte(ServerSeed))
	h.Write([]byte(strconv.FormatInt(gameId, 10)))
	hash := hex.EncodeToString(h.Sum(nil))

	// Use first 8 bytes of hash to generate crash point
	seed := hash[:8]
	crashPoint := calculateCrashPoint(seed)

	return GameResult{
		GameID:     gameId,
		CrashPoint: crashPoint,
		Hash:       hash,
		Seed:       seed,
	}
}

func calculateCrashPoint(seed string) float64 {
	// Convert hex seed to bytes
	hash := sha256.Sum256([]byte(seed))

	// Use first 8 bytes as uint64
	num := binary.BigEndian.Uint64(hash[:8])

	// Generate float in range [0, 1)
	f := float64(num) / float64(math.MaxUint64)

	// Generate uniform distribution between 1 and 10
	result := 1.0 + (f * 9.0)

	// Round to 2 decimal places
	return math.Floor(result*100) / 100
}

func CalculateCrashPoint(seed string) float64 {
	// Convert seed to int64
	n, _ := strconv.ParseInt(seed, 16, 64)

	// Generate float in range [0, 1)
	f := float64(n) / float64(1<<52)

	// Calculate crash point with house edge (2%)
	result := math.Max(100, (100*0.98)/(1-f))

	return math.Floor(result) / 100
}

func VerifyGameHash(gameID string, hash string) struct {
	Valid              bool    `json:"valid"`
	ExpectedCrashPoint float64 `json:"expectedCrashPoint"`
	Seed               string  `json:"seed"`
} {
	h := hmac.New(sha256.New, []byte(ServerSeed))
	h.Write([]byte(gameID))
	expectedHash := hex.EncodeToString(h.Sum(nil))

	seed := expectedHash[:8]
	crashPoint := calculateCrashPoint(seed)

	return struct {
		Valid              bool    `json:"valid"`
		ExpectedCrashPoint float64 `json:"expectedCrashPoint"`
		Seed               string  `json:"seed"`
	}{
		Valid:              hash == expectedHash,
		ExpectedCrashPoint: crashPoint,
		Seed:               seed,
	}
}

func VerifyGame(gameId int64, hash string) bool {
	// Generate hash for verification
	h := hmac.New(sha256.New, []byte(ServerSeed))
	h.Write([]byte(strconv.FormatInt(gameId, 10)))
	expectedHash := hex.EncodeToString(h.Sum(nil))

	return hash == expectedHash
}
