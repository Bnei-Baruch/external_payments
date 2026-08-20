package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"external_payments/db"
	"external_payments/pelecard"
	"external_payments/types"
)

// BaseUrl returns the external_payments base URL from EXT_BASE_URL env var,
// defaulting to https://checkout.kbb1.com.
func BaseUrl() string {
	if u := os.Getenv("EXT_BASE_URL"); u != "" {
		return u
	}
	return "https://checkout.kbb1.com"
}

func LogMessage(message string) {
	errLogger := gin.DefaultErrorWriter
	_, _ = errLogger.Write([]byte(message + "\n"))
}

// Gone answers a retired endpoint. The route stays registered so remaining
// callers fail loudly and identifiably instead of getting a 404.
func Gone(c *gin.Context) {
	LogMessage(fmt.Sprintf("GONE: %s %s ip=%s ua=%q referer=%q",
		c.Request.Method, c.Request.URL.Path, c.ClientIP(),
		c.Request.UserAgent(), c.Request.Referer()))

	js, _ := json.Marshal(map[string]string{
		"status": "error",
		"error":  "endpoint removed",
	})
	c.Writer.WriteHeader(http.StatusGone)
	_, _ = c.Writer.Write(js)
}

func LoadPeleCardForm(c *gin.Context) (form types.PeleCardResponse) {
	form.PelecardTransactionId = c.PostForm("PelecardTransactionId")
	form.PelecardStatusCode = c.PostForm("PelecardStatusCode")
	form.ConfirmationKey = c.PostForm("ConfirmationKey")
	form.Token = c.PostForm("Token")
	form.ApprovalNo = c.PostForm("ApprovalNo")
	form.ParamX = c.PostForm("ParamX")
	form.UserKey = c.PostForm("UserKey")

	return
}

func OnRedirectURL(url string, msg string, status string, c *gin.Context) {
	var target string
	if msg == "" {
		target = url
	} else {
		var q string
		if strings.ContainsRune(url, '?') {
			q = "&"
		} else {
			q = "?"
		}

		target = fmt.Sprintf("%s%serror=%s", url, q, msg)
	}
	html := "<script>window.location = '" + target + "';</script>"
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(html))
}

// ErrorJson answers with a status matching the cause. The body shape is
// unchanged: callers read {"status":"error"} rather than the code.
//
// Charge outcomes deliberately keep http.StatusOK. This service cannot yet
// tell a declined card from a gateway outage, and VH treats a non-2xx as a
// gateway error — which still falls back token to EMV, but raises a Sentry
// exception. Every decline would page someone. Once decline codes are
// structured, the outage case can move to 502.
func ErrorJson(status int, message string, c *gin.Context) {
	msg := map[string]string{
		"status": "error",
		"error":  message,
	}
	js, _ := json.Marshal(msg)
	c.Writer.WriteHeader(status)
	_, _ = c.Writer.Write(js)
}

func ResultJson(msg map[string]string, c *gin.Context) {
	js, _ := json.Marshal(msg)
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(js)
}

func OnSuccessPayment(url string, msg string, token string, authNo string, c *gin.Context) {
	var target string
	if msg == "" {
		target = fmt.Sprintf("%s?token=%s&authNo=%s", url, token, authNo)
	} else {
		var q string
		if strings.ContainsRune(url, '?') {
			q = "&"
		} else {
			q = "?"
		}

		target = fmt.Sprintf("%s%ssuccess=1&token=%s&authNo=%s&%s", url, q, token, authNo, msg)
	}
	html := "<script>window.location = '" + target + "';</script>"
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(html))
}

func ErrorPayment(c *gin.Context) {
	var err error
	form := LoadPeleCardForm(c)
	m := fmt.Sprintf("ErrorPayment: %+v", form)
	LogMessage(m)
	db.SetStatus(form.UserKey, "error")
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		m := fmt.Sprintf("ErrorPayment: UpdateRequestTemp %s", err.Error())
		LogMessage(m)

		ErrorJson(http.StatusOK, err.Error(), c)
		return
	}

	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		m := fmt.Sprintf("ErrorPayment: LoadRequest %s", err.Error())
		LogMessage(m)
		ErrorJson(http.StatusOK, err.Error(), c)
		return
	}
	OnRedirectURL(request.ErrorURL, pelecard.GetMessage(form.PelecardStatusCode), "error", c)
}

func CancelPayment(c *gin.Context) {
	var err error

	form := LoadPeleCardForm(c)
	db.SetStatus(form.UserKey, "cancel")
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		ErrorJson(http.StatusOK, err.Error(), c)
		return
	}

	var request = types.PaymentRequest{}
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		ErrorJson(http.StatusOK, err.Error(), c)
		return
	}
	OnRedirectURL(request.CancelURL, "", "cancel", c)
}

func OnRedirect(url string, msg string, status string, c *gin.Context) {
	var target string
	if msg == "" {
		target = url
	} else {
		var q string
		if strings.ContainsRune(url, '?') {
			q = "&"
		} else {
			q = "?"
		}

		target = fmt.Sprintf("%s%serror=%s", url, q, msg)
	}
	result := map[string]string{
		"status": status,
		"url":    target,
	}
	ResultJson(result, c)
}

func OnSuccessToken(url string, form types.PeleCardResponse, response types.PaymentResponse, c *gin.Context) {
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte("<script>window.location = '"))
	_, _ = c.Writer.Write([]byte(url))
	_, _ = c.Writer.Write([]byte("?token="))
	_, _ = c.Writer.Write([]byte(form.Token))
	_, _ = c.Writer.Write([]byte("&paramX="))
	_, _ = c.Writer.Write([]byte(form.ParamX))
	_, _ = c.Writer.Write([]byte("&CardHebrewName="))
	_, _ = c.Writer.Write([]byte(response.CardHebrewName))
	_, _ = c.Writer.Write([]byte("&CreditCardBrand="))
	_, _ = c.Writer.Write([]byte(response.CreditCardBrand))
	_, _ = c.Writer.Write([]byte("&CreditCardCompanyIssuer="))
	_, _ = c.Writer.Write([]byte(response.CreditCardCompanyIssuer))
	_, _ = c.Writer.Write([]byte("&CreditCardNumber="))
	_, _ = c.Writer.Write([]byte(response.CreditCardNumber))
	_, _ = c.Writer.Write([]byte("&CreditCardExpDate="))
	_, _ = c.Writer.Write([]byte(response.CreditCardExpDate))
	_, _ = c.Writer.Write([]byte("&CreditCardCompanyClearer="))
	_, _ = c.Writer.Write([]byte(response.CreditCardCompanyClearer))
	_, _ = c.Writer.Write([]byte("';</script>"))
}
