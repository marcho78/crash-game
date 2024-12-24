package server

import (
	"strings"
	"time"

	"crash-game/internal/auth"
	"crash-game/internal/security"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

func (s *GameServer) securityMiddleware() gin.HandlerFunc {
	limiter := security.NewIPRateLimiter(rate.Every(time.Second), 10)

	return func(c *gin.Context) {
		// Rate limiting
		ip := c.ClientIP()
		limiter := limiter.GetLimiter(ip)
		if !limiter.Allow() {
			c.JSON(429, gin.H{"error": "too many requests"})
			c.Abort()
			return
		}

		// Security headers
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Content-Security-Policy", "default-src 'self'")

		c.Next()
	}
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "no authorization header"})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix and any trailing characters like "~"
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		tokenString = strings.TrimSpace(tokenString)

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set("userId", claims.UserID)
		c.Next()
	}
}
