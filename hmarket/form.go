package hmarket

import (
	"io"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"external_payments/types"
)

// hebrewAliases maps Hebrew label keys to English field IDs.
// Elementor sends the field label as webhook key when a label is set.
var hebrewAliases = map[string]string{
	"שם":    "name",
	"אימייל": "email",
	"מייל":   "email",
	"טלפון":  "phone",
}

// parseElementorFields parses Elementor URL-encoded payload.
// Keys arrive as "אין תווית {id}" (Hebrew prefix + ASCII id) or as plain Hebrew labels.
// If the last space-separated token starts with a letter (a-z/A-Z), use it as field ID.
// Otherwise apply hebrewAliases on the full key.
func parseElementorFields(body string) map[string]string {
	fields := make(map[string]string)
	for _, pair := range strings.Split(body, "&") {
		idx := strings.IndexByte(pair, '=')
		if idx < 0 {
			continue
		}
		rawKey, rawVal := pair[:idx], pair[idx+1:]
		key, _ := url.QueryUnescape(strings.ReplaceAll(rawKey, "+", " "))
		value, _ := url.QueryUnescape(strings.ReplaceAll(rawVal, "+", " "))
		parts := strings.Fields(key)
		last := parts[len(parts)-1]
		var id string
		if len(last) > 0 && ((last[0] >= 'a' && last[0] <= 'z') || (last[0] >= 'A' && last[0] <= 'Z')) {
			id = last
		} else if alias, ok := hebrewAliases[key]; ok {
			id = alias
		} else {
			id = key
		}
		fields[id] = value
	}
	return fields
}

func splitName(full string) (first, last string) {
	parts := strings.SplitN(strings.TrimSpace(full), " ", 2)
	first = parts[0]
	if len(parts) > 1 {
		last = parts[1]
	}
	return
}

// Form handles Elementor Pro webhook for the HMarket landing page form.
// Fields: name (→ first_name + last_name), email, phone → hmarket_users
//         event, source → hmarket_activities (name, source)
func Form(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "read error"})
		return
	}

	fields := parseElementorFields(string(body))
	log.Printf("[hmarket/form] raw body: %s", string(body))
	log.Printf("[hmarket/form] parsed fields: %v", fields)

	rawPhone := fields["phone"]
	uniqPhone := normalizePhone(rawPhone)
	firstName, lastName := splitName(fields["name"])
	email := fields["email"]
	eventName := fields["event"]
	source := fields["source"]

	if rawPhone == "" && email == "" {
		log.Printf("[hmarket/form] missing phone and email")
		c.JSON(400, gin.H{"error": "phone or email is required"})
		return
	}

	user := types.HMarketUser{
		FirstName: firstName,
		LastName:  lastName,
		Email:     email,
		Phone:     strPtr(rawPhone),
		UniqPhone: strPtr(uniqPhone),
	}

	userID, isNew, _, _, err := dbUpsertUser(user)
	if err != nil {
		log.Printf("[hmarket/form] upsert user error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}
	log.Printf("[hmarket/form] user_id=%d is_new=%v email=%s phone=%s", userID, isNew, email, rawPhone)

	err = dbCreateActivity(types.HMarketActivity{
		UserID:    userID,
		Source:    source,
		Name:      eventName,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
	})
	if err != nil {
		log.Printf("[hmarket/form] create activity error: %v", err)
		c.JSON(500, gin.H{"error": "db error"})
		return
	}

	c.JSON(200, gin.H{"status": "ok", "user_id": userID, "is_new": isNew})
}
