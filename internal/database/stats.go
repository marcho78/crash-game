package database

type UserStats struct {
	UserID       string  `json:"userId"`
	TotalBets    int     `json:"totalBets"`
	TotalWagered float64 `json:"totalWagered"`
	TotalWon     float64 `json:"totalWon"`
	BiggestWin   float64 `json:"biggestWin"`
	HighestCrash float64 `json:"highestCrash"`
}

func (d *Database) GetLeaderboard(timeFrame string) ([]UserStats, error) {
	var timeFilter string
	switch timeFrame {
	case "daily":
		timeFilter = "AND created_at > NOW() - INTERVAL '24 hours'"
	case "weekly":
		timeFilter = "AND created_at > NOW() - INTERVAL '7 days'"
	case "monthly":
		timeFilter = "AND created_at > NOW() - INTERVAL '30 days'"
	default:
		timeFilter = ""
	}

	query := `
		SELECT 
			user_id,
			COUNT(*) as total_bets,
			SUM(amount) as total_wagered,
			SUM(COALESCE(win_amount, 0)) as total_won,
			MAX(win_amount) as biggest_win,
			MAX(CASE WHEN cashed_out THEN cashout_multiplier ELSE 0 END) as highest_crash
		FROM bets
		WHERE 1=1 ` + timeFilter + `
		GROUP BY user_id
		ORDER BY total_won DESC
		LIMIT 100
	`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []UserStats
	for rows.Next() {
		var s UserStats
		if err := rows.Scan(&s.UserID, &s.TotalBets, &s.TotalWagered, &s.TotalWon, &s.BiggestWin, &s.HighestCrash); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, nil
}
