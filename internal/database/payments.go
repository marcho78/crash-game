package database

import (
	"errors"

	"crash-game/internal/models"
)

func (d *Database) CreateWithdrawal(userID string, withdrawal *models.Withdrawal) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Check balance
	var balance float64
	err = tx.QueryRow("SELECT balance FROM users WHERE id = $1 FOR UPDATE", userID).Scan(&balance)
	if err != nil {
		return err
	}

	if balance < withdrawal.Amount {
		return errors.New("insufficient balance")
	}

	// Create withdrawal record
	_, err = tx.Exec(`
        INSERT INTO withdrawals (id, user_id, amount, status)
        VALUES ($1, $2, $3, $4)`,
		withdrawal.ID, userID, withdrawal.Amount, withdrawal.Status)
	if err != nil {
		return err
	}

	// Update user balance
	_, err = tx.Exec(`
        UPDATE users SET balance = balance - $1 WHERE id = $2`,
		withdrawal.Amount, userID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *Database) CreateDeposit(userID string, req *models.DepositRequest) (*string, error) {
	// Generate deposit address or payment details
	// This is where you'd integrate with your payment processor

	_, err := d.db.Exec(`
        INSERT INTO deposits (user_id, amount, status, payment_method_id)
        VALUES ($1, $2, 'pending', $3)`,
		userID, req.Amount, req.PaymentMethodID)

	if err != nil {
		return nil, err
	}

	// Return deposit address or payment details
	depositAddress := "your-deposit-address"
	return &depositAddress, nil
}
