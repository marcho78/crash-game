package database

import (
	"crash-game/internal/models"
	"fmt"
)

func (d *Database) GetDashboardStats() (*models.DashboardStats, error) {
	stats := &models.DashboardStats{}

	// Get 24h stats using a transaction
	tx, err := d.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Total and active users
	err = tx.QueryRow(`
        SELECT 
            (SELECT COUNT(*) FROM users),
            (SELECT COUNT(DISTINCT user_id) FROM bets WHERE created_at > NOW() - INTERVAL '24 hours')
    `).Scan(&stats.TotalUsers, &stats.ActiveUsers24h)
	if err != nil {
		return nil, err
	}

	// Betting stats
	err = tx.QueryRow(`
        SELECT 
            COUNT(*),
            COALESCE(SUM(amount), 0),
            COALESCE(SUM(CASE WHEN win_amount > amount THEN amount - win_amount ELSE amount END), 0),
            COALESCE(AVG(CASE WHEN cashed_out THEN cashout_multiplier ELSE 0 END), 0)
        FROM bets 
        WHERE created_at > NOW() - INTERVAL '24 hours'
    `).Scan(
		&stats.TotalBets24h,
		&stats.TotalVolume24h,
		&stats.HouseProfit24h,
		&stats.AverageMultiplier,
	)
	if err != nil {
		return nil, err
	}

	// Pending withdrawals and recent deposits
	err = tx.QueryRow(`
        SELECT 
            (SELECT COUNT(*) FROM withdrawals WHERE status = 'pending'),
            (SELECT COALESCE(SUM(amount), 0) FROM deposits WHERE status = 'completed' AND created_at > NOW() - INTERVAL '24 hours')
    `).Scan(&stats.PendingWithdraws, &stats.TotalDeposits24h)
	if err != nil {
		return nil, err
	}

	return stats, tx.Commit()
}

func (d *Database) GetUserManagementData(filters map[string]interface{}, page, limit int) ([]models.UserManagementData, int, error) {
	offset := (page - 1) * limit
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argCount := 1

	// Build dynamic where clause based on filters
	if status, ok := filters["status"].(string); ok {
		whereClause += fmt.Sprintf(" AND status = $%d", argCount)
		args = append(args, status)
		argCount++
	}

	query := fmt.Sprintf(`
        SELECT 
            u.id,
            u.username,
            COUNT(b.id) as total_bets,
            COALESCE(SUM(b.amount), 0) as total_wagered,
            COALESCE(SUM(b.win_amount - b.amount), 0) as net_profit,
            u.last_login,
            u.status,
            u.verification_level,
            ARRAY(SELECT note FROM user_notes WHERE user_id = u.id ORDER BY created_at DESC LIMIT 5) as recent_notes
        FROM users u
        LEFT JOIN bets b ON u.id = b.user_id
        %s
        GROUP BY u.id
        ORDER BY u.created_at DESC
        LIMIT $%d OFFSET $%d`, whereClause, argCount, argCount+1)

	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.UserManagementData
	for rows.Next() {
		var user models.UserManagementData
		err := rows.Scan(
			&user.UserID,
			&user.Username,
			&user.TotalBets,
			&user.TotalWagered,
			&user.NetProfit,
			&user.LastLogin,
			&user.Status,
			&user.VerificationLevel,
			&user.Notes,
		)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}

	// Get total count
	var total int
	countQuery := fmt.Sprintf(`
        SELECT COUNT(*) FROM users u %s`, whereClause)
	err = d.db.QueryRow(countQuery, args[:len(args)-2]...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return users, total, nil
}
