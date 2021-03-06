// once: go mod init external_payments
// go build -o external_payments main/* && strip external_payments && upx -9 external_payments && cp external_payments /media/sf_D_DRIVE/projects/bbpriority/
// curl -X POST -H "Content-Type: application/json" -d @request.json https://checkout.kbb1.com/payments/new

package main

import (
	"fmt"
	"log"
	"os"

	_ "github.com/gin-contrib/cors"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"

	"external_payments/db"
)

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

	_ = db.Connect()
	defer db.Disconnect()

	r := gin.Default()
	// configure to automatically detect scheme and host
	// - use http when default scheme cannot be determined
	// - use localhost:8080 when default host cannot be determined
	r.Use(location.Default())
	r.Use(CORSMiddleware())
	router(r)
	// log.Fatal(autotls.Run(r, ":"+port))
	fmt.Printf("<<< Waiting on port %s >>>\n", port)
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
		payments.GET("/confirm", ConfirmPayment)
		payments.POST("/confirm", ConfirmPayment)
	}

	paypal := r.Group("/paypal")
	{
		paypal.GET("/confirm", ConfirmPaypal)
		paypal.POST("/confirm", ConfirmPaypal)
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
