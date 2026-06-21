package hmarket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"external_payments/types"
)

type shopifyAddress struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Address1  string `json:"address1"`
	Address2  string `json:"address2"`
	City      string `json:"city"`
	Country   string `json:"country"`
}

type shopifyCustomer struct {
	Email      string          `json:"email"`
	FirstName  string          `json:"first_name"`
	LastName   string          `json:"last_name"`
	Phone      string          `json:"phone"`
	DefaultAddress *shopifyAddress `json:"default_address"`
}

type shopifyLineItem struct {
	Title     string `json:"title"`
	ProductID int64  `json:"product_id"`
	SKU       string `json:"sku"`
}

type shopifyCheckout struct {
	CartToken             string           `json:"cart_token"`
	Email                 string           `json:"email"`
	Phone                 string           `json:"phone"`
	CompletedAt           *string          `json:"completed_at"`
	BuyerAcceptsMarketing bool             `json:"buyer_accepts_marketing"`
	BillingAddress        *shopifyAddress  `json:"billing_address"`
	Customer              *shopifyCustomer `json:"customer"`
	LineItems             []shopifyLineItem `json:"line_items"`
}

func verifyShopifySignature(body []byte, signature string) bool {
	secret := os.Getenv("SHOPIFY_SECRET")
	if secret == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// Shopify handles Shopify checkout webhook (checkouts/update).
// GET is accepted for webhook registration verification.
func Shopify(c *gin.Context) {
	if c.Request.Method == "GET" {
		c.JSON(200, gin.H{"status": "ok"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "read error"})
		return
	}

	sig := c.GetHeader("X-Shopify-Hmac-Sha256")
	if !verifyShopifySignature(body, sig) {
		log.Printf("[hmarket/shopify] invalid signature")
		c.JSON(401, gin.H{"error": "invalid signature"})
		return
	}

	log.Printf("[hmarket/shopify] raw body: %s", string(body))

	var checkout shopifyCheckout
	if err := json.Unmarshal(body, &checkout); err != nil {
		log.Printf("[hmarket/shopify] parse error: %v", err)
		c.JSON(400, gin.H{"error": "invalid JSON"})
		return
	}

	log.Printf("[hmarket/shopify] cart=%s completed_at=%v email=%s phone=%s billing=%+v",
		checkout.CartToken, checkout.CompletedAt, checkout.Email, checkout.Phone, checkout.BillingAddress)

	if checkout.CompletedAt == nil || *checkout.CompletedAt == "" {
		c.JSON(200, gin.H{"status": "ignored"})
		return
	}

	source := c.GetHeader("X-Shopify-Shop-Domain")

	// Extract contact info: prefer billing_address, fall back to customer
	var firstName, lastName, rawPhone, email string
	var addr shopifyAddress

	if checkout.BillingAddress != nil {
		addr = *checkout.BillingAddress
	} else if checkout.Customer != nil && checkout.Customer.DefaultAddress != nil {
		addr = *checkout.Customer.DefaultAddress
	}

	firstName = addr.FirstName
	lastName  = addr.LastName
	rawPhone  = addr.Phone
	email     = checkout.Email

	if rawPhone == "" && checkout.Customer != nil {
		rawPhone = checkout.Customer.Phone
	}
	if email == "" && checkout.Customer != nil {
		email = checkout.Customer.Email
	}

	if rawPhone == "" && email == "" {
		log.Printf("[hmarket/shopify] no contact info in cart=%s", checkout.CartToken)
		c.JSON(200, gin.H{"status": "ignored"})
		return
	}

	uniqPhone := normalizePhone(rawPhone)

	user := types.HMarketUser{
		FirstName: firstName,
		LastName:  lastName,
		Address1:  addr.Address1,
		Address2:  addr.Address2,
		City:      addr.City,
		Country:   addr.Country,
		Email:     email,
		Phone:     strPtr(rawPhone),
		UniqPhone: strPtr(uniqPhone),
		Subscribed: checkout.BuyerAcceptsMarketing,
	}

	userID, isNew, subChanged, newSubStatus, err := dbUpsertUser(user)
	if err != nil {
		log.Printf("[hmarket/shopify] upsert user error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}
	log.Printf("[hmarket/shopify] user_id=%d is_new=%v email=%s phone=%s cart=%s",
		userID, isNew, email, rawPhone, checkout.CartToken)

	if isNew && user.Subscribed {
		_ = dbCreateSubHistory(types.HMarketSubscriptionHistory{
			UserID:      userID,
			Description: "new subscriber via " + source,
			Status:      true,
			ChangeType:  "subscription",
		})
	} else if subChanged {
		_ = dbCreateSubHistory(types.HMarketSubscriptionHistory{
			UserID:      userID,
			Description: "subscription changed to " + boolStr(newSubStatus) + " via " + source,
			Status:      newSubStatus,
			ChangeType:  "subscription",
		})
	}

	createdAt := *checkout.CompletedAt
	if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
		createdAt = t.Format("2006-01-02 15:04:05")
	}

	cartToken := checkout.CartToken
	for _, item := range checkout.LineItems {
		err := dbCreateActivity(types.HMarketActivity{
			UserID:    userID,
			Source:    source,
			Name:      item.Title,
			ProductID: item.ProductID,
			SKU:       item.SKU,
			CreatedAt: createdAt,
			CartToken: &cartToken,
		})
		if err != nil {
			log.Printf("[hmarket/shopify] create activity error: %v", err)
		}
	}

	c.JSON(200, gin.H{"status": "ok", "user_id": userID, "is_new": isNew})
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
