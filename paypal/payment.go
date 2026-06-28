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
	if os.Getenv("PAYPAL_ENV") == "sandbox" {
		base = pp.APIBaseSandBox
	}
	c, err := pp.NewClient(
		os.Getenv("PAYPAL_CLIENT_ID"),
		os.Getenv("PAYPAL_CLIENT_SECRET"),
		base,
	)
	if err != nil {
		return nil, err
	}
	_, err = c.GetAccessToken(ctx)
	return c, err
}

func NewPayment(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil {
		if err = c.ShouldBind(&request); err != nil {
			utils.ErrorJson("New Bind "+err.Error(), c)
			return
		}
	}
	utils.LogMessage(fmt.Sprintf("PayPal NewPayment: %+v", request))

	if errFound, errors := validation.ValidateStruct(request); errFound {
		utils.LogMessage(fmt.Sprintf("PayPal NewPayment validation: %+v", errors))
		utils.ErrorJson("New validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	if err = db.StoreRequest(request); err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal NewPayment StoreRequest: %s", err))
		utils.ErrorJson("StoreRequest "+err.Error(), c)
		return
	}
	db.SetStatus(request.UserKey, "in-process")

	ctx := c.Request.Context()
	client, err := newClient(ctx)
	if err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal NewPayment client: %s", err))
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
		utils.LogMessage(fmt.Sprintf("PayPal NewPayment CreateOrder: %s", err))
		utils.ErrorJson("CreateOrder: "+err.Error(), c)
		return
	}

	var approveURL string
	for _, link := range order.Links {
		if link.Rel == "approve" {
			approveURL = link.Href
			break
		}
	}
	if approveURL == "" {
		utils.LogMessage("PayPal NewPayment: no approve link")
		utils.ErrorJson("PayPal: no approve link", c)
		return
	}

	paypalEnv := os.Getenv("PAYPAL_ENV")
	if paypalEnv == "" {
		paypalEnv = "live"
	}
	if err = db.SetPaypalOrderId(request.UserKey, order.ID, paypalEnv); err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal NewPayment SetPaypalOrderId: %s", err))
	}

	utils.LogMessage(fmt.Sprintf("PayPal NewPayment: order=%s approve=%s", order.ID, approveURL))
	utils.OnRedirect(approveURL, "", "success", c)
}

func GoodPayment(c *gin.Context) {
	userKey := c.Query("UserKey")
	orderID := c.Query("token") // PayPal appends ?token=ORDER_ID&PayerID=PAYER_ID to return URL

	utils.LogMessage(fmt.Sprintf("PayPal GoodPayment: userKey=%s orderID=%s", userKey, orderID))

	if userKey == "" || orderID == "" {
		utils.LogMessage("PayPal GoodPayment: missing UserKey or token")
		c.Status(http.StatusBadRequest)
		return
	}

	var request types.PaymentRequest
	if err := db.LoadRequest(userKey, &request); err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal GoodPayment LoadRequest: %s", err))
		utils.ErrorJson("load request failed", c)
		return
	}

	ctx := c.Request.Context()
	client, err := newClient(ctx)
	if err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal GoodPayment client: %s", err))
		utils.OnRedirectURL(request.ErrorURL, "PayPal client error", "error", c)
		return
	}

	capture, err := client.CaptureOrder(ctx, orderID, pp.CaptureOrderRequest{})
	if err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal GoodPayment CaptureOrder: %s", err))
		utils.OnRedirectURL(request.ErrorURL, "capture failed", "error", c)
		return
	}

	if capture.Status != "COMPLETED" {
		utils.LogMessage(fmt.Sprintf("PayPal GoodPayment: capture status=%s", capture.Status))
		utils.OnRedirectURL(request.ErrorURL, "capture not completed: "+capture.Status, "error", c)
		return
	}

	captureID := capture.ID
	if len(capture.PurchaseUnits) > 0 && capture.PurchaseUnits[0].Payments != nil &&
		len(capture.PurchaseUnits[0].Payments.Captures) > 0 {
		captureID = capture.PurchaseUnits[0].Payments.Captures[0].ID
	}

	paymentDate := time.Now().Format("2006-01-02 15:04:05")
	utils.LogMessage(fmt.Sprintf("PayPal GoodPayment: captureID=%s date=%s", captureID, paymentDate))

	paypalEnv := os.Getenv("PAYPAL_ENV")
	if paypalEnv == "" {
		paypalEnv = "live"
	}
	if err = db.StorePaypalCapture(request, captureID, paymentDate, paypalEnv); err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal GoodPayment StorePaypalCapture: %s", err))
	}

	db.SetStatus(userKey, "valid")

	sep := "?"
	if strings.ContainsRune(request.GoodURL, '?') {
		sep = "&"
	}
	target := fmt.Sprintf("%s%ssuccess=1&transaction_id=%s&order_id=%s", request.GoodURL, sep, captureID, orderID)
	html := "<script>window.location = '" + target + "';</script>"
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(html))
}

func ErrorPayment(c *gin.Context) {
	userKey := c.Query("UserKey")
	utils.LogMessage(fmt.Sprintf("PayPal ErrorPayment: userKey=%s", userKey))
	db.SetStatus(userKey, "error")

	var request types.PaymentRequest
	if err := db.LoadRequest(userKey, &request); err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal ErrorPayment LoadRequest: %s", err))
		utils.ErrorJson("load request failed", c)
		return
	}
	utils.OnRedirectURL(request.ErrorURL, "PayPal error", "error", c)
}

func CancelPayment(c *gin.Context) {
	userKey := c.Query("UserKey")
	utils.LogMessage(fmt.Sprintf("PayPal CancelPayment: userKey=%s", userKey))
	db.SetStatus(userKey, "cancel")

	var request types.PaymentRequest
	if err := db.LoadRequest(userKey, &request); err != nil {
		utils.LogMessage(fmt.Sprintf("PayPal CancelPayment LoadRequest: %s", err))
		utils.ErrorJson("load request failed", c)
		return
	}
	utils.OnRedirectURL(request.CancelURL, "", "cancel", c)
}

// Confirm handles the legacy CiviCRM PayPal confirmation endpoint.
func Confirm(c *gin.Context) {
	var err error
	request := types.PaypalRegister{}
	if err = c.ShouldBindJSON(&request); err != nil {
		if err = c.ShouldBindQuery(&request); err != nil {
			utils.ErrorJson("Bind "+err.Error(), c)
			return
		}
	}
	db.StorePaypal(request)
	c.Status(http.StatusOK)
}
