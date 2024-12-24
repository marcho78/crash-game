package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"crash-game/internal/game"

	"github.com/gin-gonic/gin"
)

type VerificationData struct {
	GameID       int64   `json:"gameId"`
	CrashedPoint float64 `json:"crashPoint"`
	Hash         string  `json:"hash"`
	Seed         string  `json:"seed"`
}

func (s *GameServer) VerifyGame(c *gin.Context) {
	var data VerificationData
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(400, gin.H{"error": "invalid input"})
		return
	}

	// Verify hash
	h := hmac.New(sha256.New, []byte(game.ServerSeed))
	h.Write([]byte(strconv.FormatInt(data.GameID, 10)))
	expectedHash := hex.EncodeToString(h.Sum(nil))

	if data.Hash != expectedHash {
		c.JSON(400, gin.H{"error": "invalid hash"})
		return
	}

	// Verify crash point
	expectedCrashPoint := game.CalculateCrashPoint(data.Seed)
	if data.CrashedPoint != expectedCrashPoint {
		c.JSON(400, gin.H{"error": "invalid crash point"})
		return
	}

	c.JSON(200, gin.H{
		"valid":      true,
		"gameId":     data.GameID,
		"crashPoint": data.CrashedPoint,
	})
}
