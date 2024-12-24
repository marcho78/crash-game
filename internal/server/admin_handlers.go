package server

import (
	"crash-game/internal/models"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *GameServer) adminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		adminID := c.GetInt("adminId")
		if adminID == 0 {
			c.JSON(401, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *GameServer) GetPendingWithdrawals(c *gin.Context) {
	withdrawals, err := s.db.GetPendingWithdrawals()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get withdrawals"})
		return
	}
	c.JSON(200, withdrawals)
}

func (s *GameServer) HandleWithdrawalApproval(c *gin.Context) {
	adminID := c.GetInt("adminId")
	var req models.WithdrawalApproval

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	if req.Action == "approve" {
		err := s.db.ApproveWithdrawal(adminID, req.WithdrawalID)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	} else {
		if req.RejectionReason == "" {
			c.JSON(400, gin.H{"error": "rejection reason required"})
			return
		}
		err := s.db.RejectWithdrawal(adminID, req.WithdrawalID, req.RejectionReason)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(200, gin.H{"message": "withdrawal " + req.Action + "d successfully"})
}

func (s *GameServer) GetAdminActions(c *gin.Context) {
	actions, err := s.db.GetAdminActions()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get admin actions"})
		return
	}
	c.JSON(200, actions)
}

func (s *GameServer) GetDashboardStats(c *gin.Context) {
	stats, err := s.db.GetDashboardStats()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get dashboard stats"})
		return
	}
	c.JSON(200, stats)
}

func (s *GameServer) GetUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	filters := map[string]interface{}{
		"status":             c.Query("status"),
		"verification_level": c.Query("verification_level"),
	}

	users, total, err := s.db.GetUserManagementData(filters, page, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get users"})
		return
	}

	c.JSON(200, gin.H{
		"users": users,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

func (s *GameServer) UpdateUserStatus(c *gin.Context) {
	userID := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"`
		Note   string `json:"note"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}

	if err := s.db.UpdateUserStatus(userID, req.Status, req.Note); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Create notification for status change
	s.notificationManager.CreateNotification(&models.AdminNotification{
		Type:     "user_status_change",
		Priority: "medium",
		Message:  fmt.Sprintf("User %s status changed to %s", userID, req.Status),
	})

	c.JSON(200, gin.H{"message": "user status updated"})
}

func (s *GameServer) GetAdminNotifications(c *gin.Context) {
	adminID := c.GetString("adminId")
	notifications, err := s.db.GetAdminNotifications(adminID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get notifications"})
		return
	}
	c.JSON(200, notifications)
}

func (s *GameServer) MarkNotificationRead(c *gin.Context) {
	notificationID := c.Param("id")
	adminID := c.GetString("adminId")

	if err := s.db.MarkNotificationRead(notificationID, adminID); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "notification marked as read"})
}
