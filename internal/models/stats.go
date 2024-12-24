package models

type DashboardStats struct {
	TotalUsers        int     `json:"totalUsers"`
	ActiveUsers24h    int     `json:"activeUsers24h"`
	TotalBets24h      int     `json:"totalBets24h"`
	TotalVolume24h    float64 `json:"totalVolume24h"`
	HouseProfit24h    float64 `json:"houseProfit24h"`
	PendingWithdraws  int     `json:"pendingWithdraws"`
	TotalDeposits24h  float64 `json:"totalDeposits24h"`
	AverageMultiplier float64 `json:"averageMultiplier"`
	OnlineUsers       int     `json:"onlineUsers"`
}

type UserManagementData struct {
	UserID            string   `json:"userId"`
	Username          string   `json:"username"`
	TotalBets         int      `json:"totalBets"`
	TotalWagered      float64  `json:"totalWagered"`
	NetProfit         float64  `json:"netProfit"`
	LastLogin         string   `json:"lastLogin"`
	Status            string   `json:"status"`
	VerificationLevel string   `json:"verificationLevel"`
	Notes             []string `json:"notes"`
}

type AdminNotification struct {
	ID        int    `json:"id"`
	Type      string `json:"type"`     // withdrawal_request, suspicious_activity, etc.
	Priority  string `json:"priority"` // high, medium, low
	Message   string `json:"message"`
	Read      bool   `json:"read"`
	CreatedAt string `json:"createdAt"`
}
