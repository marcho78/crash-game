package models

import "time"

type AdminUser struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	LastLogin time.Time `json:"lastLogin"`
}

type WithdrawalApproval struct {
	WithdrawalID    int    `json:"withdrawalId"`
	Action          string `json:"action" binding:"required,oneof=approve reject"`
	RejectionReason string `json:"rejectionReason,omitempty"`
}

type AdminAction struct {
	ID         int         `json:"id"`
	AdminID    int         `json:"adminId"`
	ActionType string      `json:"actionType"`
	TargetType string      `json:"targetType"`
	TargetID   string      `json:"targetId"`
	Details    interface{} `json:"details"`
	CreatedAt  time.Time   `json:"createdAt"`
}
