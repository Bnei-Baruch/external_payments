package hmarket

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

const logFile = "/tmp/hmarket.log"

func logToFile(headers http.Header, body string) {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[hmarket] cannot open log file: %v", err)
		return
	}
	defer f.Close()

	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(f, "--- %s ---\n", ts)
	for k, v := range headers {
		fmt.Fprintf(f, "%s: %v\n", k, v)
	}
	fmt.Fprintf(f, "body: %s\n\n", body)
}

// Webhook receives and logs arbitrary payloads from HMarket.
func Webhook(c *gin.Context) {
	body, _ := io.ReadAll(c.Request.Body)
	logToFile(c.Request.Header, string(body))
	c.JSON(200, gin.H{"status": "ok"})
}
