package hmarket

import (
	"log"

	"github.com/gin-gonic/gin"

	"external_payments/db"
	"external_payments/types"
)

type blacklistRequest struct {
	UserID      int64  `json:"user_id"`
	Description string `json:"description"`
	Blacklist   bool   `json:"blacklist"`
}

func Blacklist(c *gin.Context) {
	var req blacklistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	found, err := db.BlacklistHMarketUser(req.UserID, req.Blacklist)
	if err != nil {
		log.Printf("[hmarket/blacklist] update error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}
	if !found {
		c.JSON(404, gin.H{"error": "user not found"})
		return
	}

	_ = db.CreateHMarketSubscriptionHistory(types.HMarketSubscriptionHistory{
		UserID:      req.UserID,
		Description: req.Description,
		Status:      req.Blacklist,
		ChangeType:  "blacklist",
	})

	log.Printf("[hmarket/blacklist] user_id=%d blacklist=%v", req.UserID, req.Blacklist)
	c.JSON(200, gin.H{"status": "ok"})
}
