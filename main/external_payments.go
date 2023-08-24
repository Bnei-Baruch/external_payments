// once: go mod init external_payments
// CGO_ENABLED=0 go build -o external_payments main/* && strip external_payments && upx -9 external_payments && cp external_payments /media/sf_D_DRIVE/projects/bbpriority/
// curl -X POST -H "Content-Type: application/json" -d @request.json https://checkout.kbb1.com/payments/new

package main

import (
	"fmt"
	"html/template"
	"log"
	"os"

	_ "github.com/gin-contrib/cors"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"external_payments/counters"
	"external_payments/db"
	"external_payments/emv"
	"external_payments/payment"
	"external_payments/token"
)

func main() {
	env := os.Getenv("ENV")
	isProd := false
	if env == "" {
		env = "production"
	}
	if env == "production" {
		isProd = true
		gin.SetMode(gin.ReleaseMode)
	}

	port := os.Getenv("EXT_PORT")
	if port == "" {
		port = ":8080"
	}

	if isProd {
		_ = db.Connect()
		defer db.Disconnect()
	}

	r := gin.Default()
	// configure to automatically detect scheme and host
	// - use http when default scheme cannot be determined
	// - use localhost:8080 when default host cannot be determined
	r.Use(location.Default())
	r.Use(CORSMiddleware())
	router(r, isProd)
	fmt.Printf("<<< Waiting on port %s >>>\n", port)
	log.Fatal(r.Run(":" + port))
}

func router(r *gin.Engine, isProd bool) {
	// Request for payment
	payments := r.Group("/payments")
	{
		// regular payment
		payments.GET("/new", payment.NewPayment)
		payments.POST("/new", payment.NewPayment)
		payments.POST("/good", payment.GoodPayment)
		payments.POST("/error", payment.ErrorPayment)
		payments.POST("/cancel", payment.CancelPayment)
		payments.GET("/confirm", payment.ConfirmPayment)
		payments.POST("/confirm", payment.ConfirmPayment)
		payments.GET("/transaction", payment.GetTransaction)
		payments.POST("/transaction", payment.GetTransaction)
	}
	withToken := r.Group("/token")
	{
		// recurrent payments with token
		withToken.GET("/new", token.NewPayment)
		withToken.POST("/new", token.NewPayment)
		withToken.POST("/good", token.GoodPayment)
		withToken.POST("/error", token.ErrorPayment)
		withToken.POST("/cancel", token.CancelPayment)
		withToken.GET("/confirm", token.ConfirmPayment)
		withToken.POST("/confirm", token.ConfirmPayment)
		withToken.GET("/charge", token.Charge)
		withToken.POST("/charge", token.Charge)
		withToken.POST("/chargex", token.ChargeX)
		withToken.POST("/refund", token.Refund)
		withToken.POST("/authorize", token.AuthorizeCC)
		withToken.POST("/authorizex", token.AuthorizeCCX)
		withToken.POST("/authorizerecurr", token.AuthorizeCCRecurr)
	}
	withEmv := r.Group("/emv")
	{
		// recurrent payments with token
		withEmv.GET("/new", emv.NewPayment)
		withEmv.POST("/new", emv.NewPayment)
		withEmv.POST("/good", emv.GoodPayment)
		withEmv.POST("/error", emv.ErrorPayment)
		withEmv.POST("/cancel", emv.CancelPayment)
		withEmv.GET("/confirm", emv.ConfirmPayment)
		withEmv.POST("/confirm", emv.ConfirmPayment)
		withEmv.GET("/charge", emv.Charge)
		withEmv.POST("/charge", emv.Charge)
	}

	paypal := r.Group("/paypal")
	{
		paypal.GET("/confirm", ConfirmPaypal)
		paypal.POST("/confirm", ConfirmPaypal)
	}

	projects := r.Group("/projects/:language/:project_name")
	{
		r.SetFuncMap(template.FuncMap{
			"formatAmount": formatAmount,
		})
		r.LoadHTMLFiles("templates/counter.tmpl", "templates/statistics.tmpl", "templates/404.html")
		projects.GET("/counter", counters.Counter)
		projects.GET("/statistics", counters.Statistics)
	}
	r.Static("/assets", "./assets")

	//for _, route := range r.Routes() {
	//	fmt.Println(route.Method, route.Path)
	//}
}

func formatAmount(number float64) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%.0f", number)
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
