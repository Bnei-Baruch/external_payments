package hmarket

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"external_payments/db"
	"external_payments/types"
)

type wcBilling struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Company   string `json:"company"`
	Address1  string `json:"address_1"`
	Address2  string `json:"address_2"`
	City      string `json:"city"`
	Country   string `json:"country"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

type wcLineItem struct {
	Name      string `json:"name"`
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
}

type wcOrder struct {
	DateCreated string       `json:"date_created"`
	Billing     wcBilling    `json:"billing"`
	LineItems   []wcLineItem `json:"line_items"`
}

// overridable in tests
var (
	dbUpsertUser       = db.UpsertHMarketUser
	dbCreateActivity   = db.CreateHMarketActivity
	dbCreateSubHistory = db.CreateHMarketSubscriptionHistory
)

var nonDigits = regexp.MustCompile(`\D`)

func normalizePhone(phone string) string {
	digits := nonDigits.ReplaceAllString(phone, "")
	if digits == "" {
		return ""
	}
	if strings.HasPrefix(digits, "0") {
		digits = "972" + digits[1:]
	}
	return digits
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func HW1(c *gin.Context) {
	var order wcOrder
	if err := c.ShouldBindJSON(&order); err != nil {
		log.Printf("[hmarket] hw1 parse error: %v", err)
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	source    := c.GetHeader("X-Wc-Webhook-Source")
	rawPhone  := order.Billing.Phone
	uniqPhone := normalizePhone(rawPhone)

	log.Printf("[hmarket/hw1] source=%s date=%s email=%s phone=%s uniq_phone=%s",
		source, order.DateCreated, order.Billing.Email, rawPhone, uniqPhone)

	// convert "2026-06-02T15:04:05" → "2026-06-02 15:04:05" for MySQL
	createdAt := order.DateCreated
	if t, err := time.Parse("2006-01-02T15:04:05", order.DateCreated); err == nil {
		createdAt = t.Format("2006-01-02 15:04:05")
	}

	user := types.HMarketUser{
		FirstName:   order.Billing.FirstName,
		LastName:    order.Billing.LastName,
		Company:     order.Billing.Company,
		Address1:    order.Billing.Address1,
		Address2:    order.Billing.Address2,
		City:        order.Billing.City,
		Country:     order.Billing.Country,
		Email:       order.Billing.Email,
		Phone:       strPtr(rawPhone),
		UniqPhone:   strPtr(uniqPhone),
		Subscribed:  false,
		Blacklisted: false,
	}

	userID, subChanged, newSubStatus, err := dbUpsertUser(user)
	if err != nil {
		log.Printf("[hmarket/hw1] upsert user error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}
	log.Printf("[hmarket/hw1] user_id=%d sub_changed=%v new_sub=%v", userID, subChanged, newSubStatus)

	if subChanged {
		_ = dbCreateSubHistory(types.HMarketSubscriptionHistory{
			UserID:      userID,
			Description: fmt.Sprintf("subscription changed to %v due to %s", newSubStatus, source),
			Status:      newSubStatus,
			ChangeType:  "subscription",
		})
	}

	for _, item := range order.LineItems {
		log.Printf("[hmarket/hw1] activity user_id=%d product_id=%d name=%q", userID, item.ProductID, item.Name)
		err := dbCreateActivity(types.HMarketActivity{
			UserID:    userID,
			Source:    source,
			Name:      item.Name,
			ProductID: item.ProductID,
			SKU:       item.SKU,
			CreatedAt: createdAt,
		})
		if err != nil {
			log.Printf("[hmarket/hw1] create activity error: %v", err)
		}
	}

	c.JSON(200, gin.H{"status": "ok"})
}
