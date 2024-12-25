package database

import (
	"crash-game/internal/models"
	"database/sql"
	"errors"
	"log"

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
	log.Printf("DB: Saving game - ID: %s, Hash: %s", game.GameID, game.Hash)

	result, err := d.db.Exec(`
		INSERT INTO games (game_id, crash_point, start_time, end_time, hash)
		VALUES ($1::uuid, $2, $3, $4, $5)
	`, game.GameID, game.CrashPoint, game.StartTime, game.EndTime, game.Hash)

	if err != nil {
		log.Printf("DB: Failed to save game: %v", err)
		return err
	}

	rows, _ := result.RowsAffected()
	log.Printf("DB: Game saved successfully, rows affected: %d", rows)
	return nil
}

func (d *Database) GetGameHistory(userID string) ([]models.GameHistory, error) {
	rows, err := d.db.Query(`
			SELECT g.game_id, g.crash_point, g.start_time, g.end_time, g.hash
			FROM games g
			LEFT JOIN bets b ON g.game_id = b.game_id AND b.user_id = $1::uuid
			ORDER BY g.start_time DESC
			LIMIT 50
		`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []models.GameHistory
	for rows.Next() {
		var h models.GameHistory
		err := rows.Scan(&h.GameID, &h.CrashPoint, &h.StartTime, &h.EndTime, &h.Hash)
		if err != nil {
			return nil, err
		}
		history = append(history, h)
	}
	return history, nil
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

func (d *Database) SaveGameHistory(history *models.GameHistory) error {
	log.Printf("📝 Starting SaveGameHistory for game %s with %d players",
		history.GameID, len(history.Players))

	tx, err := d.db.Begin()
	if err != nil {
		log.Printf("❌ Failed to begin transaction: %v", err)
		return err
	}
	defer tx.Rollback()

	// Insert game first
	result, err := tx.Exec(`
		INSERT INTO games (game_id, crash_point, start_time, end_time, hash, status)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
	`, history.GameID, history.CrashPoint, history.StartTime, history.EndTime, history.Hash, history.Status)

	if err != nil {
		log.Printf("❌ Failed to insert game: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("✅ Game inserted: ID=%s, Status=%s, Rows=%d",
		history.GameID, history.Status, rowsAffected)

	// Insert bets
	for _, player := range history.Players {
		log.Printf("👤 Inserting bet - Game: %s, User: %s, Amount: %.2f",
			history.GameID, player.UserID, player.BetAmount)

		result, err = tx.Exec(`
			INSERT INTO bets (game_id, user_id, amount, win_amount, cashed_out, cashout_at, auto_cashout)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
		`, history.GameID, player.UserID, player.BetAmount, player.WinAmount,
			player.CashedOut, player.CashoutAt, player.AutoCashout)

		if err != nil {
			log.Printf("❌ Failed to insert bet for user %s: %v", player.UserID, err)
			return err
		}

		rowsAffected, _ = result.RowsAffected()
		log.Printf("✅ Bet inserted: Game=%s, User=%s, Amount=%.2f, Win=%.2f, Rows=%d",
			history.GameID, player.UserID, player.BetAmount, player.WinAmount, rowsAffected)
	}

	if err := tx.Commit(); err != nil {
		log.Printf("❌ Failed to commit transaction: %v", err)
		return err
	}

	log.Printf("✅ Successfully saved game history for %s with %d players",
		history.GameID, len(history.Players))
	return nil
}

func (d *Database) GetGameByID(gameID string) (*models.GameHistory, error) {
	log.Printf("DB: Looking up game %s", gameID)
	var game models.GameHistory

	err := d.db.QueryRow(`
		SELECT game_id, crash_point, start_time, end_time, hash
		FROM games 
		WHERE game_id = $1::uuid
	`, gameID).Scan(&game.GameID, &game.CrashPoint, &game.StartTime, &game.EndTime, &game.Hash)

	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("DB: No game found with ID %s", gameID)
		} else {
			log.Printf("DB: Error querying game: %v", err)
		}
		return nil, err
	}

	log.Printf("DB: Found game - ID: %s, Hash: %s, CrashPoint: %f",
		game.GameID, game.Hash, game.CrashPoint)
	return &game, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) GetPlayerGameHistory(userID string) ([]models.GameHistory, error) {
	log.Printf("🔍 Getting game history for user: %s", userID)

	query := `
		SELECT g.game_id, g.crash_point, g.hash, g.start_time, g.end_time, g.status,
			   b.amount as bet_amount, b.win_amount, b.cashed_out, 
			   b.cashout_at, 
			   b.auto_cashout
		FROM games g
		JOIN bets b ON g.game_id = b.game_id
		WHERE b.user_id = $1
		ORDER BY g.start_time DESC`

	rows, err := d.db.Query(query, userID)
	if err != nil {
		log.Printf("❌ Failed to query game history: %v", err)
		return nil, err
	}
	defer rows.Close()

	var history []models.GameHistory
	for rows.Next() {
		var game models.GameHistory
		var cashoutAt sql.NullTime
		var autoCashout sql.NullFloat64

		err := rows.Scan(
			&game.GameID,
			&game.CrashPoint,
			&game.Hash,
			&game.StartTime,
			&game.EndTime,
			&game.Status,
			&game.BetAmount,
			&game.WinAmount,
			&game.CashedOut,
			&cashoutAt,
			&autoCashout,
		)
		if err != nil {
			log.Printf("❌ Failed to scan row: %v", err)
			return nil, err
		}

		if cashoutAt.Valid {
			game.CashoutAt = float64(cashoutAt.Time.Unix())
		}
		if autoCashout.Valid {
			game.AutoCashout = autoCashout.Float64
		}

		history = append(history, game)
	}

	log.Printf("✅ Found %d games in history for user %s", len(history), userID)
	return history, nil
}

func (d *Database) GetPlayerBetHistory(userID string) (*sql.Rows, error) {
	query := `
		SELECT 
			b.game_id,
			b.amount as bet_amount,
			b.win_amount,
			b.cashed_out,
			b.cashout_multiplier,
			b.auto_cashout,
			b.created_at,
			b.cashout_at,
			g.crash_point,
			g.hash,
			g.status
		FROM bets b
		JOIN games g ON b.game_id = g.game_id
		WHERE b.user_id = $1
		ORDER BY b.created_at DESC
		LIMIT 50
	`
	return d.db.Query(query, userID)
}

// GetDB returns the underlying database connection
func (d *Database) GetDB() *sql.DB {
	return d.db
}
