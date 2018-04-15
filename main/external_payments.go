// go build -o external_payments main/*; strip external_payments ; cp external_payments /media/sf_projects/bbpriority/
// curl -X POST -H "Content-Type: application/json" -d @request.json https://checkout.kbb1.com/payments/new
package main

import (
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/gin-contrib/cors"
	_ "github.com/joho/godotenv/autoload"

	"os"
	"github.com/gshilin/external_payments/db"
	"log"
	"fmt"
)

// TODO:
// TODO Add reference [and other fields???]
// TODO Approve result
// TODO Approve requesting client
// TODO: On successful payment -- redirect to ErrorURL in case of validation errors

func main() {
	env := os.Getenv("ENV")
	if env == "" {
		env = "production"
	}
	if env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	port := os.Getenv("EXT_PORT")
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
	// log.Fatal(autotls.Run(r, ":"+port))
	fmt.Printf("Waiting on port %s\n", port)
	log.Fatal(r.Run(":" + port))
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
