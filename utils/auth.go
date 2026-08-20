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

// RequireAPIClient rejects a caller that does not present a valid token in
// "Authorization: Bearer <token>". It always enforces — a route guarded with
// this is never open, whatever is or is not configured.
func RequireAPIClient() gin.HandlerFunc {
	return func(c *gin.Context) {
		if client, ok := resolveClient(c); ok {
			c.Set(APIClientKey, client)
			c.Next()
			return
		}

		LogMessage(fmt.Sprintf("AUTH DENIED: %s %s ip=%s ua=%q referer=%q token_present=%t",
			c.Request.Method, c.Request.URL.Path, c.ClientIP(),
			c.Request.UserAgent(), c.Request.Referer(), presentedToken(c) != ""))
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
	}
}

// ObserveAPIClient resolves a token if one is presented and lets every request
// through either way, logging callers that did not authenticate.
//
// This is the first half of guarding a route that has callers we cannot
// enumerate: deploy in observe mode, read the log to find out who is really
// out there, issue their keys, then switch the route to RequireAPIClient. The
// alternative — enforcing first — takes payments down for whoever we forgot.
//
// A route left in observe mode is unprotected. It is a deployment step, not a
// resting state.
func ObserveAPIClient() gin.HandlerFunc {
	return func(c *gin.Context) {
		if client, ok := resolveClient(c); ok {
			c.Set(APIClientKey, client)
			c.Next()
			return
		}

		LogMessage(fmt.Sprintf("AUTH OBSERVE: %s %s ip=%s ua=%q referer=%q token_present=%t",
			c.Request.Method, c.Request.URL.Path, c.ClientIP(),
			c.Request.UserAgent(), c.Request.Referer(), presentedToken(c) != ""))
		c.Next()
	}
}

func presentedToken(c *gin.Context) string {
	return strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
}

func resolveClient(c *gin.Context) (db.APIClient, bool) {
	token := presentedToken(c)
	if token == "" {
		return db.APIClient{}, false
	}

	if client, ok := db.LookupAPIClient(token); ok {
		return client, true
	}

	// Single shared secret for our own services. They are one caller each with
	// no organization of their own, so a row per service would be bookkeeping
	// without benefit; the table exists for the many-client case, where the key
	// has to carry the organization.
	if internal := os.Getenv("INTERNAL_API_TOKEN"); internal != "" &&
		subtle.ConstantTimeCompare([]byte(token), []byte(internal)) == 1 {
		return db.APIClient{Name: "internal"}, true
	}

	return db.APIClient{}, false
}

// APIClientFor returns the authenticated client, if the route resolved one.
func APIClientFor(c *gin.Context) (db.APIClient, bool) {
	v, ok := c.Get(APIClientKey)
	if !ok {
		return db.APIClient{}, false
	}
	client, ok := v.(db.APIClient)
	return client, ok
}

// ResolveOrganization decides which organization a request belongs to, and is
// how two generations of the WooCommerce plugin coexist.
//
//	old plugin  sends Organization in the body, no token
//	new plugin  sends a token whose key carries the organization, no body field
//
// The key wins when there is one, because the caller cannot choose it. The body
// value is accepted otherwise, so sites that have not taken the plugin update
// keep working.
//
// Call this after binding and before validating: Organization is a required
// field, so a request from the new plugin fails validation unless the value is
// filled in first.
//
// A caller that sends both and disagrees is reported. That means either an old
// site that has been issued a key, or a new one still sending a stale setting —
// worth finding during the transition, not worth rejecting the payment over.
func ResolveOrganization(c *gin.Context, fromBody string) string {
	client, ok := APIClientFor(c)
	if !ok || client.Organization == "" {
		return fromBody
	}

	if fromBody != "" && fromBody != client.Organization {
		LogMessage(fmt.Sprintf(
			"ORG MISMATCH: client=%q key=%s body=%s — using the key. %s %s ip=%s",
			client.Name, client.Organization, fromBody,
			c.Request.Method, c.Request.URL.Path, c.ClientIP()))
	}

	return client.Organization
}
