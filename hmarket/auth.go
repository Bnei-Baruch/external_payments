package hmarket

import (
	"crypto/subtle"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := os.Getenv("HMARKET_API_TOKEN")
		if token == "" {
			c.Next()
			return
		}

		// Bearer, for scripted callers.
		if h := c.GetHeader("Authorization"); strings.HasPrefix(h, "Bearer ") {
			if secretEqual(strings.TrimPrefix(h, "Bearer "), token) {
				c.Next()
				return
			}
		}

		// Basic, so a browser opens a login dialog rather than rendering the
		// 401 body. Any username is accepted; the password is the token.
		if _, password, ok := c.Request.BasicAuth(); ok && secretEqual(password, token) {
			c.Next()
			return
		}

		// Without this header a browser shows the JSON instead of prompting.
		c.Header("WWW-Authenticate", `Basic realm="hmarket", charset="UTF-8"`)
		c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
	}
}

// secretEqual compares in constant time so a wrong value cannot be recovered by
// timing repeated requests.
func secretEqual(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}
