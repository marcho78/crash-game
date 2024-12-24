package models

type Withdrawal struct {
	ID     string  `json:"id"`
	UserID string  `json:"userId"`
	Amount float64 `json:"amount"`
	Status string  `json:"status"`
}
