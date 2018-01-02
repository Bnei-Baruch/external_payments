package main

import (
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/gin-contrib/cors"
	_ "github.com/joho/godotenv/autoload"

	"os"
	"github.com/gshilin/external_payments/db"
)

// +Request for payment
// Store parameters
// Replace return url to us
// Redirect to pelecard
// When redirected back: record cause of redirection, record additional parameters and redirect to requester

// When requested approval -- send it

// TODO:
// TODO Approve requesting client

func main() {
	env := os.Getenv("ENV")
	if env == "" {
		env = "production"
	}
	if env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}

	db.Connect()
	defer db.Disconnect()

	r := gin.Default()
	// configure to automatically detect scheme and host
	// - use http when default scheme cannot be determined
	// - use localhost:8080 when default host cannot be determined
	r.Use(location.Default())
	r.Use(CORSMiddleware())
	router(r)
	r.Run(port)
}

func router(r *gin.Engine) {
	// Request for payment
	payments := r.Group("/payments")
	{
		payments.GET("/new", NewPayment)
		payments.POST("/new", NewPayment)
		payments.POST("/good", GoodPayment)
		payments.POST("/error", ErrorPayment)
		payments.POST("/cancel", CancelPayment)
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(200)
		} else {
			c.Next()
		}
	}
}

// Helpers

// Does array s includes value e?
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
