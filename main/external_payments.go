// once: go mod init external_payments
// CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o external_payments main/* && upx -9 --force-macos external_payments
// curl -X POST -H "Content-Type: application/json" -d @request.json https://checkout.kbb1.com/payments/new

package main

import (
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	_ "github.com/gin-contrib/cors"
	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"external_payments/counters"
	"external_payments/db"
	"external_payments/emv"
	"external_payments/hmarket"
	"external_payments/payment"
	paypalhandler "external_payments/paypal"
	renewcard "external_payments/renew-card"
	"external_payments/token"
	"external_payments/utils"
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

	r := gin.New()
	r.Use(gin.LoggerWithFormatter(accessLogFormatter))
	r.Use(gin.Recovery())
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
	renew := r.Group("/renew")
	{
		// regular payment
		renew.POST("/renew-card", renewcard.RenewCard)
		renew.POST("/good", renewcard.GoodJ2)
		renew.POST("/error", utils.ErrorPayment)
		renew.POST("/cancel", utils.CancelPayment)
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
		// GET retired: a charge over GET is reachable by CSRF, prefetch and link
		// scanners, and leaks the request into logs and Referer headers.
		withToken.GET("/charge", utils.Gone)
		withToken.POST("/charge", token.Charge)
		withToken.POST("/chargex", token.ChargeX)
		withToken.POST("/refund", token.Refund)
		// Retired: unauthenticated card-validity probes. Routed to Gone so any
		// remaining caller is identified in the log; delete after 2026-09-16.
		withToken.POST("/authorize", utils.Gone)
		withToken.POST("/authorizex", utils.Gone)
		withToken.POST("/authorizerecurr", utils.Gone)
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
		// GET retired — see /token/charge above.
		withEmv.GET("/charge", utils.Gone)
		withEmv.POST("/charge", emv.Charge)
		withEmv.GET("/new_token", emv.NewToken)
		withEmv.POST("/new_token", emv.NewToken)
		withEmv.POST("/good_token", emv.GoodToken)
	}

	hmarketGroup := r.Group("/hmarket")
	{
		hmarketGroup.POST("/webhook", hmarket.Webhook)
		hmarketGroup.POST("/hw1", hmarket.HW1)
		hmarketGroup.POST("/form", hmarket.Form)
		hmarketGroup.GET("/shopify", hmarket.Shopify)
		hmarketGroup.POST("/shopify", hmarket.Shopify)
		hmarketGroup.GET("/status", hmarket.Status)
	}
	hmarketAuth := r.Group("/hmarket", hmarket.AuthMiddleware())
	{
		hmarketAuth.GET("/export", hmarket.Export)
		hmarketAuth.GET("/subscription-status", hmarket.SubscriptionStatus)
		hmarketAuth.POST("/blacklist", hmarket.Blacklist)
		hmarketAuth.GET("/audiences", hmarket.Audiences)
	}

	withPaypal := r.Group("/paypal")
	{
		withPaypal.GET("/new", paypalhandler.NewPayment)
		withPaypal.POST("/new", paypalhandler.NewPayment)
		withPaypal.GET("/good", paypalhandler.GoodPayment)
		withPaypal.GET("/error", paypalhandler.ErrorPayment)
		withPaypal.GET("/cancel", paypalhandler.CancelPayment)
		withPaypal.GET("/confirm", paypalhandler.Confirm)
		withPaypal.POST("/confirm", paypalhandler.Confirm)
		withPaypal.POST("/charge", paypalhandler.Charge)
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

// accessLogFormatter matches gin's default line but drops the query string.
// Callers redirect the payer's browser to /payments/new with Name, Email,
// Phone, Street and City as query parameters, so the default logger writes a
// named person's contact details and home address into the journal on every
// checkout.
func accessLogFormatter(p gin.LogFormatterParams) string {
	path := p.Path
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n",
		p.TimeStamp.Format("2006/01/02 - 15:04:05"),
		p.StatusCode, p.Latency, p.ClientIP, p.Method, path)
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
