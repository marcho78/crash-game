package database

import (
	"crash-game/internal/auth"
	"errors"
	"log"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserProfile struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	Balance      float64 `json:"balance"`
	TotalWagered float64 `json:"totalWagered"`
	TotalWon     float64 `json:"totalWon"`
	GamesPlayed  int     `json:"gamesPlayed"`
	JoinDate     string  `json:"joinDate"`
}

func (d *Database) UpdateBalance(userID string, amount float64, txType string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Debug logging
	var exists bool
	err = tx.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("user not found")
	}

	// Lock the user row for update
	var currentBalance float64
	err = tx.QueryRow(`
        SELECT balance FROM users 
        WHERE id = $1 
        FOR UPDATE`, userID).Scan(&currentBalance)
	if err != nil {
		return err
	}

	// Check if sufficient balance for debit
	if txType == "debit" && currentBalance < amount {
		return errors.New("insufficient balance")
	}

	// Update balance
	newBalance := currentBalance
	if txType == "credit" {
		newBalance += amount
	} else {
		newBalance -= amount
	}

	// Update user balance
	_, err = tx.Exec(`
        UPDATE users 
        SET balance = $1 
        WHERE id = $2`, newBalance, userID)
	if err != nil {
		return err
	}

	// Record transaction
	_, err = tx.Exec(`
        INSERT INTO transactions (user_id, amount, type, balance_after)
        VALUES ($1, $2, $3, $4)`,
		userID, amount, txType, newBalance)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *Database) GetUserProfile(userID string) (*UserProfile, error) {
	var profile UserProfile
	err := d.db.QueryRow(`
        SELECT 
            u.id,
            u.username,
            u.balance,
            COALESCE(SUM(b.amount), 0) as total_wagered,
            COALESCE(SUM(b.win_amount), 0) as total_won,
            COUNT(DISTINCT b.game_id) as games_played,
            u.created_at
        FROM users u
        LEFT JOIN bets b ON u.id = b.user_id
        WHERE u.id = $1
        GROUP BY u.id`, userID).Scan(
		&profile.ID,
		&profile.Username,
		&profile.Balance,
		&profile.TotalWagered,
		&profile.TotalWon,
		&profile.GamesPlayed,
		&profile.JoinDate,
	)
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (d *Database) CreateUser(username, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	userID := uuid.New().String()

	// Add initial balance and debug logging
	_, err = d.db.Exec(`
        INSERT INTO users (id, username, password_hash, balance)
        VALUES ($1, $2, $3, $4)`,
		userID, username, string(hashedPassword), 1000.0)
	if err != nil {
		return err
	}

	// Verify the balance was set
	var balance float64
	err = d.db.QueryRow("SELECT balance FROM users WHERE id = $1", userID).Scan(&balance)
	if err != nil {
		return err
	}
	log.Printf("Created user %s with initial balance: %f", username, balance)

	return nil
}

func (d *Database) GetUserByUsername(username string) (*auth.User, error) {
	var user auth.User
	err := d.db.QueryRow(`
        SELECT id, username, password_hash, balance, created_at 
        FROM users WHERE username = $1`,
		username).Scan(&user.ID, &user.Username, &user.Password, &user.Balance, &user.Created)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (d *Database) GetUserBalance(userID string) (float64, error) {
	var balance float64
	err := d.db.QueryRow("SELECT balance FROM users WHERE id = $1", userID).Scan(&balance)
	return balance, err
}
