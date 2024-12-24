package server

import (
	"crash-game/internal/auth"

	"github.com/gin-gonic/gin"
)

func (s *GameServer) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid input"})
		return
	}

	// Create user in database
	err := s.db.CreateUser(req.Username, req.Password)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "user created successfully"})
}

func (s *GameServer) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid input"})
		return
	}

	// Verify credentials
	user, err := s.db.GetUserByUsername(req.Username)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid credentials"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(200, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
		},
	})
}
