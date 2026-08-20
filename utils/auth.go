package utils

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"external_payments/db"
)

// APIClientKey is where the resolved client is stashed on the request context.
// Handlers read it to learn which organization the caller is scoped to, rather
// than trusting an Organization field in the request body.
const APIClientKey = "api_client"

// RequireAPIClient authenticates a caller against civicrm_bb_ext_api_clients
// using "Authorization: Bearer <token>".
//
// With neither a client row nor INTERNAL_API_TOKEN set, the route stays open and every call is logged with the
// caller's identity. That is the rollout path: deploy, watch who actually calls,
// provision their keys, then insert the first row to switch enforcement on. A
// route that silently stops working is worse than one that reports who is still
// using it — the same reason the retired routes answer 410 rather than 404.
func RequireAPIClient() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")

		if client, ok := db.LookupAPIClient(token); ok {
			c.Set(APIClientKey, client)
			c.Next()
			return
		}

		// Single shared secret for our own services. They are one caller each
		// with no organization of their own, so a row per service would be
		// bookkeeping without benefit; the table exists for the many-client
		// case, where the key has to carry the organization.
		if internal := os.Getenv("INTERNAL_API_TOKEN"); internal != "" &&
			subtle.ConstantTimeCompare([]byte(token), []byte(internal)) == 1 {
			c.Set(APIClientKey, db.APIClient{Name: "internal"})
			c.Next()
			return
		}

		if db.APIClientCount() == 0 && os.Getenv("INTERNAL_API_TOKEN") == "" {
			LogMessage(fmt.Sprintf("AUTH OPEN (no clients provisioned): %s %s ip=%s ua=%q",
				c.Request.Method, c.Request.URL.Path, c.ClientIP(), c.Request.UserAgent()))
			c.Next()
			return
		}

		LogMessage(fmt.Sprintf("AUTH DENIED: %s %s ip=%s ua=%q referer=%q token_present=%t",
			c.Request.Method, c.Request.URL.Path, c.ClientIP(),
			c.Request.UserAgent(), c.Request.Referer(), token != ""))
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}

// APIClientFor returns the authenticated client, if the route was guarded.
func APIClientFor(c *gin.Context) (db.APIClient, bool) {
	v, ok := c.Get(APIClientKey)
	if !ok {
		return db.APIClient{}, false
	}
	client, ok := v.(db.APIClient)
	return client, ok
}
