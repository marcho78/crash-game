package database

import (
	"crash-game/internal/models"
	"encoding/json"
)

func (d *Database) GetPendingWithdrawals() ([]models.WithdrawalRequest, error) {
	rows, err := d.db.Query(`
        SELECT w.id, w.user_id, w.amount, w.status, w.created_at,
               u.username, pm.type, pm.address
        FROM withdrawals w
        JOIN users u ON w.user_id = u.id
        JOIN payment_methods pm ON w.payment_method_id = pm.id
        WHERE w.status = 'pending'
        ORDER BY w.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var withdrawals []models.WithdrawalRequest
	for rows.Next() {
		var w models.WithdrawalRequest
		err := rows.Scan(&w.ID, &w.UserID, &w.Amount, &w.Status, &w.CreatedAt,
			&w.Username, &w.PaymentType, &w.PaymentAddress)
		if err != nil {
			return nil, err
		}
		withdrawals = append(withdrawals, w)
	}
	return withdrawals, nil
}

func (d *Database) ApproveWithdrawal(adminID int, withdrawalID int) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Update withdrawal status
	_, err = tx.Exec(`
        UPDATE withdrawals 
        SET status = 'approved',
            status_updated_by = $1,
            status_updated_at = CURRENT_TIMESTAMP
        WHERE id = $2 AND status = 'pending'`,
		adminID, withdrawalID)
	if err != nil {
		return err
	}

	// Log admin action
	details := map[string]interface{}{
		"action":       "approve",
		"withdrawalId": withdrawalID,
	}
	detailsJSON, _ := json.Marshal(details)

	_, err = tx.Exec(`
        INSERT INTO admin_actions (admin_id, action_type, target_type, target_id, details)
        VALUES ($1, 'withdrawal_approval', 'withdrawal', $2, $3)`,
		adminID, withdrawalID, detailsJSON)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *Database) RejectWithdrawal(adminID int, withdrawalID int, reason string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get withdrawal amount and user_id
	var amount float64
	var userID string
	err = tx.QueryRow(`
        SELECT amount, user_id FROM withdrawals 
        WHERE id = $1 AND status = 'pending'`,
		withdrawalID).Scan(&amount, &userID)
	if err != nil {
		return err
	}

	// Update withdrawal status
	_, err = tx.Exec(`
        UPDATE withdrawals 
        SET status = 'rejected',
            status_updated_by = $1,
            status_updated_at = CURRENT_TIMESTAMP,
            rejection_reason = $2
        WHERE id = $3`,
		adminID, reason, withdrawalID)
	if err != nil {
		return err
	}

	// Refund the user's balance
	_, err = tx.Exec(`
        UPDATE users 
        SET balance = balance + $1 
        WHERE id = $2`,
		amount, userID)
	if err != nil {
		return err
	}

	// Log admin action
	details := map[string]interface{}{
		"action":       "reject",
		"withdrawalId": withdrawalID,
		"reason":       reason,
	}
	detailsJSON, _ := json.Marshal(details)

	_, err = tx.Exec(`
        INSERT INTO admin_actions (admin_id, action_type, target_type, target_id, details)
        VALUES ($1, 'withdrawal_approval', 'withdrawal', $2, $3)`,
		adminID, withdrawalID, detailsJSON)
	if err != nil {
		return err
	}

	return tx.Commit()
}
