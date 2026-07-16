package paypal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// Charge handles server-side PayPal charging for subscription renewals.
// Switches on token prefix:
//   - "B-..." → PayPal billing agreement (NVP DoReferenceTransaction, legacy)
//   - anything else → PayPal vault token (REST Orders API v2)
func Charge(c *gin.Context) {
	var request types.PaymentRequest
	if err := c.BindJSON(&request); err != nil {
		if err = c.ShouldBind(&request); err != nil {
			utils.ErrorJson("Bind: "+err.Error(), c)
			return
		}
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] Charge: userKey=%s token=%s", request.UserKey, tokenPreview(request.Token)))

	if errFound, errors := validation.ValidateStruct(request); errFound {
		utils.ErrorJson("validateStruct: "+strings.Join(errors, "\n"), c)
		return
	}

	if db.FindRecentSuccessfulCharge(request.Reference) {
		utils.LogMessage(fmt.Sprintf("[PayPal] Charge: duplicate suppressed reference=%s", request.Reference))
		utils.ResultJson(map[string]string{"status": "success", "capture_id": ""}, c)
		return
	}

	if err := db.StoreRequest(request); err != nil {
		utils.ErrorJson("StoreRequest: "+err.Error(), c)
		return
	}
	db.SetStatus(request.UserKey, "in-process")

	ctx := c.Request.Context()
	var captureID string
	var err error

	if strings.HasPrefix(request.Token, "B-") {
		captureID, err = chargeNVP(request)
	} else {
		captureID, err = chargeVaultToken(ctx, request)
	}

	if err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] Charge error: %s", err))
		db.SetStatus(request.UserKey, "invalid")
		utils.ErrorJson("charge failed: "+err.Error(), c)
		return
	}

	loc, _ := time.LoadLocation("Asia/Jerusalem")
	paymentDate := time.Now().In(loc).Format("2006-01-02 15:04:05")
	env := paypalEnv()

	if err = db.StorePaypalCapture(request, captureID, paymentDate, env); err != nil {
		utils.LogMessage(fmt.Sprintf("[PayPal] Charge StorePaypalCapture error: %s", err))
	}

	db.SetStatus(request.UserKey, "valid")
	utils.LogMessage(fmt.Sprintf("[PayPal] Charge success: captureID=%s userKey=%s", captureID, request.UserKey))
	utils.ResultJson(map[string]string{
		"status":     "success",
		"capture_id": captureID,
	}, c)
}

func tokenPreview(t string) string {
	if len(t) > 8 {
		return t[:8] + "..."
	}
	return t
}

// chargeNVP charges via PayPal's legacy NVP DoReferenceTransaction API.
// Used for existing subscriptions with billing agreement IDs (B-...).
func chargeNVP(req types.PaymentRequest) (string, error) {
	nvpUser := os.Getenv("PAYPAL_NVP_USER")
	nvpPwd := os.Getenv("PAYPAL_NVP_PWD")
	nvpSig := os.Getenv("PAYPAL_NVP_SIGNATURE")
	if nvpUser == "" || nvpPwd == "" || nvpSig == "" {
		return "", fmt.Errorf("PAYPAL_NVP_USER/PWD/SIGNATURE not configured")
	}

	nvpBase := "https://api-3t.paypal.com/nvp"
	if os.Getenv("PAYPAL_ENV") == "sandbox" {
		nvpBase = "https://api-3t.sandbox.paypal.com/nvp"
	}

	currency := req.Currency
	if currency == "NIS" {
		currency = "ILS"
	}

	params := url.Values{}
	params.Set("METHOD", "DoReferenceTransaction")
	params.Set("VERSION", "108.0")
	params.Set("USER", nvpUser)
	params.Set("PWD", nvpPwd)
	params.Set("SIGNATURE", nvpSig)
	params.Set("REFERENCEID", req.Token)
	params.Set("AMT", fmt.Sprintf("%.2f", req.Price))
	params.Set("CURRENCYCODE", currency)
	params.Set("PAYMENTACTION", "Sale")
	params.Set("DESC", req.Details)

	utils.LogMessage(fmt.Sprintf("[PayPal] NVP DoReferenceTransaction: refID=%s amt=%.2f currency=%s", req.Token, req.Price, currency))

	resp, err := http.Post(nvpBase, "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("NVP request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	result, _ := url.ParseQuery(string(body))

	ack := result.Get("ACK")
	utils.LogMessage(fmt.Sprintf("[PayPal] NVP response: ACK=%s TRANSACTIONID=%s", ack, result.Get("TRANSACTIONID")))

	if ack != "Success" && ack != "SuccessWithWarning" {
		return "", fmt.Errorf("NVP error %s: %s", result.Get("L_ERRORCODE0"), result.Get("L_LONGMESSAGE0"))
	}

	return result.Get("TRANSACTIONID"), nil
}

// chargeVaultToken charges via PayPal REST API using a saved vault token.
// Used for new subscriptions created after vault support was added.
func chargeVaultToken(ctx context.Context, req types.PaymentRequest) (string, error) {
	client, err := newClient(ctx)
	if err != nil {
		return "", fmt.Errorf("PayPal client: %w", err)
	}

	currency := req.Currency
	if currency == "NIS" {
		currency = "ILS"
	}

	utils.LogMessage(fmt.Sprintf("[PayPal] Vault charge: token=%s amt=%.2f currency=%s", tokenPreview(req.Token), req.Price, currency))

	order, err := client.CreateOrder(ctx, pp.OrderIntentCapture, []pp.PurchaseUnitRequest{{
		Amount: &pp.PurchaseUnitAmount{
			Currency: currency,
			Value:    fmt.Sprintf("%.2f", req.Price),
		},
		Description: req.Details,
		CustomID:    req.UserKey,
	}}, &pp.PaymentSource{
		Token: &pp.PaymentSourceToken{
			ID:   req.Token,
			Type: "PAYMENT_METHOD_TOKEN",
		},
	}, nil)
	if err != nil {
		return "", fmt.Errorf("CreateOrder: %w", err)
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] Vault CreateOrder: id=%s status=%s", order.ID, order.Status))

	capture, err := client.CaptureOrder(ctx, order.ID, pp.CaptureOrderRequest{})
	if err != nil {
		return "", fmt.Errorf("CaptureOrder: %w", err)
	}
	utils.LogMessage(fmt.Sprintf("[PayPal] Vault CaptureOrder: status=%s", capture.Status))

	if capture.Status != "COMPLETED" {
		return "", fmt.Errorf("capture status: %s", capture.Status)
	}

	captureID := capture.ID
	if len(capture.PurchaseUnits) > 0 && capture.PurchaseUnits[0].Payments != nil &&
		len(capture.PurchaseUnits[0].Payments.Captures) > 0 {
		captureID = capture.PurchaseUnits[0].Payments.Captures[0].ID
	}
	return captureID, nil
}
