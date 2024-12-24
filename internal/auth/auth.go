package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt"
)

type User struct {
	ID       string    `json:"id"`
	Username string    `json:"username"`
	Password string    `json:"-"`
	Balance  float64   `json:"balance"`
	Created  time.Time `json:"created"`
}

type Claims struct {
	UserID string `json:"userId"`
	jwt.StandardClaims
}

const secretKey = "your-secret-key" // In production, use environment variable

func GenerateToken(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secretKey))
}

func ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
