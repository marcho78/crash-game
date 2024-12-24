package database

import (
	"crash-game/internal/models"
	"database/sql"
	"errors"

	_ "github.com/lib/pq"
)

type Database struct {
	db *sql.DB
}

func NewDatabase(connStr string) (*Database, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) SaveGame(game *models.GameHistory) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert game
	_, err = tx.Exec(`
		INSERT INTO games (game_id, crash_point, start_time, end_time, hash)
		VALUES ($1, $2, $3, $4, $5)
	`, game.GameID, game.CrashPoint, game.StartTime, game.EndTime, game.Hash)
	if err != nil {
		return err
	}

	// Insert bets
	for userId, player := range game.Players {
		_, err = tx.Exec(`
			INSERT INTO bets (game_id, user_id, amount, cashed_out, cashout_multiplier, win_amount)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, game.GameID, userId, player.BetAmount, player.CashedOut, player.CashoutAt, player.WinAmount)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *Database) GetGameHistory(limit int) ([]models.GameHistory, error) {
	rows, err := d.db.Query(`
		SELECT g.game_id::text, g.crash_point, g.start_time, g.end_time, g.hash,
			   b.user_id, b.amount, b.cashed_out, b.cashout_multiplier, b.win_amount
		FROM games g
		LEFT JOIN bets b ON g.game_id = b.game_id
		ORDER BY g.game_id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	games := make(map[string]*models.GameHistory)
	for rows.Next() {
		var gameId string
		var game models.GameHistory
		var userId string
		var bet models.Player

		err := rows.Scan(
			&gameId, &game.CrashPoint, &game.StartTime, &game.EndTime, &game.Hash,
			&userId, &bet.BetAmount, &bet.CashedOut, &bet.CashoutAt, &bet.WinAmount,
		)
		if err != nil {
			return nil, err
		}

		if _, exists := games[gameId]; !exists {
			game.GameID = gameId
			game.Players = []models.PlayerHistory{}
			games[gameId] = &game
		}

		if userId != "" {
			playerHistory := models.PlayerHistory{
				UserID:    userId,
				BetAmount: bet.BetAmount,
				CashedOut: bet.CashedOut,
				CashoutAt: bet.CashoutAt,
				WinAmount: bet.WinAmount,
			}
			games[gameId].Players = append(games[gameId].Players, playerHistory)
		}
	}

	// Convert map to slice
	result := make([]models.GameHistory, 0, len(games))
	for _, game := range games {
		result = append(result, *game)
	}

	return result, nil
}

func (d *Database) SaveNotification(notif *models.AdminNotification) error {
	_, err := d.db.Exec(`
		INSERT INTO notifications (type, priority, message, created_at)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)`,
		notif.Type, notif.Priority, notif.Message)
	return err
}

func (d *Database) CleanupOldNotifications(days int) error {
	_, err := d.db.Exec(`
		DELETE FROM notifications 
		WHERE created_at < NOW() - INTERVAL '1 day' * $1`,
		days)
	return err
}

func (d *Database) GetAdminActions() ([]models.AdminAction, error) {
	rows, err := d.db.Query(`
		SELECT id, admin_id, action_type, target_type, target_id, details, created_at 
		FROM admin_actions 
		ORDER BY created_at DESC 
		LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []models.AdminAction
	for rows.Next() {
		var action models.AdminAction
		err := rows.Scan(&action.ID, &action.AdminID, &action.ActionType,
			&action.TargetType, &action.TargetID, &action.Details, &action.CreatedAt)
		if err != nil {
			return nil, err
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func (d *Database) UpdateUserStatus(userID string, status string, note string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE users SET status = $1 WHERE id = $2`, status, userID)
	if err != nil {
		return err
	}

	if note != "" {
		_, err = tx.Exec(`
			INSERT INTO user_notes (user_id, note) 
			VALUES ($1, $2)`, userID, note)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *Database) GetAdminNotifications(adminID string) ([]models.AdminNotification, error) {
	rows, err := d.db.Query(`
		SELECT id, type, priority, message, read, created_at 
		FROM notifications 
		WHERE admin_id = $1 
		ORDER BY created_at DESC`, adminID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []models.AdminNotification
	for rows.Next() {
		var n models.AdminNotification
		err := rows.Scan(&n.ID, &n.Type, &n.Priority, &n.Message, &n.Read, &n.CreatedAt)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, nil
}

func (d *Database) MarkNotificationRead(notificationID string, adminID string) error {
	result, err := d.db.Exec(`
		UPDATE notifications 
		SET read = true 
		WHERE id = $1 AND admin_id = $2`,
		notificationID, adminID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("notification not found")
	}
	return nil
}

func (d *Database) SaveGameHistory(history models.GameHistory) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Parse string UUID to UUID type
	_, err = tx.Exec(`
		INSERT INTO games (game_id, crash_point, start_time, end_time, hash)
		VALUES ($1::uuid, $2, $3, $4, $5)
	`, history.GameID, history.CrashPoint, history.StartTime, history.EndTime, history.Hash)
	if err != nil {
		return err
	}

	for _, player := range history.Players {
		_, err = tx.Exec(`
			INSERT INTO bets (game_id, user_id, amount, cashed_out, cashout_multiplier, win_amount)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
		`, history.GameID, player.UserID, player.BetAmount, player.CashedOut, player.CashoutAt, player.WinAmount)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
