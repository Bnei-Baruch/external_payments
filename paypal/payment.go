package paypal

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	pp "github.com/plutov/paypal/v4"

	"external_payments/db"
	"external_payments/types"
	"external_payments/utils"
	"external_payments/validation"
)

func newClient(ctx context.Context) (*pp.Client, error) {
	base := pp.APIBaseLive
	env := os.Getenv("PAYPAL_ENV")
	if env == "sandbox" {
		base = pp.APIBaseSandBox
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] newClient env=%s base=%s", env, base))
	clientID := os.Getenv("PAYPAL_CLIENT_ID")
	utils.LogMessage(fmt.Sprintf("[PayPal] newClient clientID=%s (len=%d)", clientID[:min(8, len(clientID))]+"...", len(clientID)))
	c, err := pp.NewClient(clientID, os.Getenv("PAYPAL_CLIENT_SECRET"), base)
	if err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] newClient NewClient error: %v", err))
		return nil, err
	}
	token, err := c.GetAccessToken(ctx)
	if err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] newClient GetAccessToken error: %v", err))
		return nil, err
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] newClient token_type=%s", token.Type))
	return c, nil
}

func paypalEnv() string {
	env := os.Getenv("PAYPAL_ENV")
	if env == "" {
		return "live"
	}
	return env
}

func NewPayment(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil {
		if err = c.ShouldBind(&request); err != nil {
			utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment bind error: %s", err))
			utils.ErrorJson("New Bind "+err.Error(), c)
			return
		}
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment request: %+v", request))

	if errFound, errors := validation.ValidateStruct(request); errFound {
		utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment validation errors: %+v", errors))
		utils.ErrorJson("New validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	if err = db.StoreRequest(request); err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment StoreRequest error: %s", err))
		utils.ErrorJson("StoreRequest "+err.Error(), c)
		return
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment stored request userKey=%s", request.UserKey))
	db.SetStatus(request.UserKey, "in-process")

	ctx := c.Request.Context()
	client, err := newClient(ctx)
	if err != nil {
		utils.ErrorJson("PayPal client: "+err.Error(), c)
		return
	}

	currency := request.Currency
	if currency == "NIS" {
		currency = "ILS"
	}
	price := fmt.Sprintf("%.2f", request.Price)
	baseURL := utils.BaseUrl()
	returnURL := fmt.Sprintf("%s/paypal/good?UserKey=%s", baseURL, request.UserKey)
	cancelURL := fmt.Sprintf("%s/paypal/cancel?UserKey=%s", baseURL, request.UserKey)

	utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment CreateOrder: currency=%s price=%s returnURL=%s cancelURL=%s",
		currency, price, returnURL, cancelURL))

	order, err := client.CreateOrder(ctx,
		pp.OrderIntentCapture,
		[]pp.PurchaseUnitRequest{{
			Amount: &pp.PurchaseUnitAmount{
				Currency: currency,
				Value:    price,
			},
			Description: request.Details,
			CustomID:    request.UserKey,
		}},
		nil,
		&pp.ApplicationContext{
			ReturnURL: returnURL,
			CancelURL: cancelURL,
		},
	)
	if err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment CreateOrder error: %s", err))
		utils.ErrorJson("CreateOrder: "+err.Error(), c)
		return
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment order created: id=%s status=%s links=%+v",
		order.ID, order.Status, order.Links))

	var approveURL string
	for _, link := range order.Links {
		if link.Rel == "approve" {
			approveURL = link.Href
			break
		}
	}
	if approveURL == "" {
		utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment no approve link in order links: %+v", order.Links))
		utils.ErrorJson("PayPal: no approve link", c)
		return
	}

	env := paypalEnv()
	if err = db.SetPaypalOrderId(request.UserKey, order.ID, env); err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment SetPaypalOrderId error: %s", err))
	} else {
		utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment saved orderID=%s env=%s userKey=%s", order.ID, env, request.UserKey))
	}

	utils.LogMessage(fmt.Sprintf("[PayPal] NewPayment redirecting to approveURL=%s", approveURL))
	utils.OnRedirect(approveURL, "", "success", c)
}

func GoodPayment(c *gin.Context) {
	userKey := c.Query("UserKey")
	orderID := c.Query("token") // PayPal appends ?token=ORDER_ID&PayerID=PAYER_ID to return URL
	payerID := c.Query("PayerID")

	utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment: userKey=%s orderID=%s payerID=%s fullURL=%s",
		userKey, orderID, payerID, c.Request.URL.String()))

	if userKey == "" || orderID == "" {
		utils.LogMessage("[PayPal] GoodPayment: missing UserKey or token — bad request")
		c.Status(http.StatusBadRequest)
		return
	}

	var request types.PaymentRequest
	if err := db.LoadRequest(userKey, &request); err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment LoadRequest error: %s", err))
		utils.ErrorJson("load request failed", c)
		return
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment loaded request: org=%s price=%f currency=%s goodURL=%s",
		request.Organization, request.Price, request.Currency, request.GoodURL))

	ctx := c.Request.Context()
	client, err := newClient(ctx)
	if err != nil {
		utils.OnRedirectURL(request.ErrorURL, "PayPal client error", "error", c)
		return
	}

	utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment calling CaptureOrder orderID=%s", orderID))
	capture, err := client.CaptureOrder(ctx, orderID, pp.CaptureOrderRequest{})
	if err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment CaptureOrder error: %s", err))
		utils.OnRedirectURL(request.ErrorURL, "capture failed", "error", c)
		return
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment CaptureOrder response: status=%s id=%s purchaseUnits=%+v",
		capture.Status, capture.ID, capture.PurchaseUnits))

	if capture.Status != "COMPLETED" {
		utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment capture not completed: status=%s", capture.Status))
		utils.OnRedirectURL(request.ErrorURL, "capture not completed: "+capture.Status, "error", c)
		return
	}

	captureID := capture.ID
	if len(capture.PurchaseUnits) > 0 && capture.PurchaseUnits[0].Payments != nil &&
		len(capture.PurchaseUnits[0].Payments.Captures) > 0 {
		captureID = capture.PurchaseUnits[0].Payments.Captures[0].ID
		utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment captureID from PurchaseUnits: %s", captureID))
	} else {
		utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment captureID fallback to capture.ID: %s", captureID))
	}

	loc, _ := time.LoadLocation("Asia/Jerusalem")
	paymentDate := time.Now().In(loc).Format("2006-01-02 15:04:05")
	env := paypalEnv()
	utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment storing capture: captureID=%s date=%s env=%s", captureID, paymentDate, env))

	if err = db.StorePaypalCapture(request, captureID, paymentDate, env); err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment StorePaypalCapture error: %s", err))
	} else {
		utils.LogMessage("[PayPal] GoodPayment StorePaypalCapture OK")
	}

	db.SetStatus(userKey, "valid")
	utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment status set to valid userKey=%s", userKey))

	sep := "?"
	if strings.ContainsRune(request.GoodURL, '?') {
		sep = "&"
	}
	target := fmt.Sprintf("%s%ssuccess=1&transaction_id=%s&order_id=%s", request.GoodURL, sep, captureID, orderID)
	utils.LogMessage(fmt.Sprintf("[PayPal] GoodPayment redirecting to: %s", target))
	html := "<script>window.location = '" + target + "';</script>"
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(html))
}

func ErrorPayment(c *gin.Context) {
	userKey := c.Query("UserKey")
	utils.LogMessage(fmt.Sprintf("[PayPal] ErrorPayment: userKey=%s fullURL=%s", userKey, c.Request.URL.String()))
	db.SetStatus(userKey, "error")

	var request types.PaymentRequest
	if err := db.LoadRequest(userKey, &request); err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] ErrorPayment LoadRequest error: %s", err))
		utils.ErrorJson("load request failed", c)
		return
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] ErrorPayment redirecting to errorURL=%s", request.ErrorURL))
	utils.OnRedirectURL(request.ErrorURL, "PayPal error", "error", c)
}

func CancelPayment(c *gin.Context) {
	userKey := c.Query("UserKey")
	utils.LogMessage(fmt.Sprintf("[PayPal] CancelPayment: userKey=%s fullURL=%s", userKey, c.Request.URL.String()))
	db.SetStatus(userKey, "cancel")

	var request types.PaymentRequest
	if err := db.LoadRequest(userKey, &request); err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] CancelPayment LoadRequest error: %s", err))
		utils.ErrorJson("load request failed", c)
		return
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] CancelPayment redirecting to cancelURL=%s", request.CancelURL))
	utils.OnRedirectURL(request.CancelURL, "", "cancel", c)
}

// Confirm handles the legacy CiviCRM PayPal confirmation endpoint.
func Confirm(c *gin.Context) {
	var err error
	request := types.PaypalRegister{}
	if err = c.ShouldBindJSON(&request); err != nil {
		if err = c.ShouldBindQuery(&request); err != nil {
			utils.LogMessage(fmt.Sprintf("[PayPal] Confirm bind error: %s", err))
			utils.ErrorJson("Bind "+err.Error(), c)
			return
		}
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] Confirm: %+v", request))
	db.StorePaypal(request)
	c.Status(http.StatusOK)
}

