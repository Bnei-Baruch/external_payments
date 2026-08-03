package paypal

import (
	"context"
	"fmt"
	"log"

	pp "github.com/plutov/paypal/v4"

	"external_payments/types"
)

// Custom structs for PayPal Vault v3 (not in plutov/paypal v4 library).

type vaultOrderRequest struct {
	Intent        string                   `json:"intent"`
	PurchaseUnits []pp.PurchaseUnitRequest `json:"purchase_units"`
	PaymentSource vaultPaymentSource       `json:"payment_source"`
}

type vaultPaymentSource struct {
	Paypal vaultPaypalSource `json:"paypal"`
}

type vaultPaypalSource struct {
	ExperienceContext vaultExperienceContext `json:"experience_context"`
	Attributes        vaultAttributes       `json:"attributes"`
}

type vaultExperienceContext struct {
	ReturnURL string `json:"return_url"`
	CancelURL string `json:"cancel_url"`
}

type vaultAttributes struct {
	Vault vaultConfig `json:"vault"`
}

type vaultConfig struct {
	StoreInVault string `json:"store_in_vault"`
	UsageType    string `json:"usage_type"`
}

type vaultOrderResponse struct {
	ID    string          `json:"id"`
	Links []pp.Link `json:"links"`
}

type vaultCaptureResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	PurchaseUnits []struct {
		Payments *struct {
			Captures []struct {
				ID string `json:"id"`
			} `json:"captures,omitempty"`
		} `json:"payments,omitempty"`
	} `json:"purchase_units,omitempty"`
	PaymentSource *struct {
		Paypal *struct {
			Attributes *struct {
				Vault *struct {
					ID string `json:"id"`
				} `json:"vault,omitempty"`
			} `json:"attributes,omitempty"`
		} `json:"paypal,omitempty"`
	} `json:"payment_source,omitempty"`
}

// createVaultOrder creates a PayPal order with vault instruction so the customer's
// PayPal account is saved for future server-side charges.
func createVaultOrder(ctx context.Context, client *pp.Client, req types.PaymentRequest, returnURL, cancelURL string) (approveURL, orderID string, err error) {
	currency := req.Currency
	if currency == "NIS" {
		currency = "ILS"
	}

	body := vaultOrderRequest{
		Intent: pp.OrderIntentCapture,
		PurchaseUnits: []pp.PurchaseUnitRequest{{
			Amount: &pp.PurchaseUnitAmount{
				Currency: currency,
				Value:    fmt.Sprintf("%.2f", req.Price),
			},
			Description: req.Details,
			CustomID:    req.UserKey,
			InvoiceID:   req.Reference,
		}},
		PaymentSource: vaultPaymentSource{
			Paypal: vaultPaypalSource{
				ExperienceContext: vaultExperienceContext{
					ReturnURL: returnURL,
					CancelURL: cancelURL,
				},
				Attributes: vaultAttributes{
					Vault: vaultConfig{
						StoreInVault: "ON_SUCCESS",
						UsageType:    "MERCHANT",
					},
				},
			},
		},
	}

	httpReq, err := client.NewRequest(ctx, "POST", fmt.Sprintf("%s/v2/checkout/orders", client.APIBase), body)
	if err != nil {
		return
	}

	var resp vaultOrderResponse
	if err = client.SendWithAuth(httpReq, &resp); err != nil {
		return
	}

	orderID = resp.ID
	log.Printf("[PayPal] vault order id=%s links=%+v", orderID, resp.Links)
	for _, link := range resp.Links {
		if link.Rel == "approve" || link.Rel == "payer-action" {
			approveURL = link.Href
			break
		}
	}
	if approveURL == "" {
		err = fmt.Errorf("no approve link in vault order response (id=%s links=%v)", orderID, resp.Links)
	}
	return
}

// captureVaultOrder captures a vault-enabled PayPal order and returns the capture ID
// and the vault token (if PayPal stored the payment method successfully).
func captureVaultOrder(ctx context.Context, client *pp.Client, orderID string) (captureID, vaultToken string, err error) {
	httpReq, err := client.NewRequest(ctx, "POST", fmt.Sprintf("%s/v2/checkout/orders/%s/capture", client.APIBase, orderID), nil)
	if err != nil {
		return
	}

	var resp vaultCaptureResponse
	if err = client.SendWithAuth(httpReq, &resp); err != nil {
		return
	}

	if resp.Status != "COMPLETED" {
		err = fmt.Errorf("capture status: %s", resp.Status)
		return
	}

	captureID = resp.ID
	if len(resp.PurchaseUnits) > 0 && resp.PurchaseUnits[0].Payments != nil &&
		len(resp.PurchaseUnits[0].Payments.Captures) > 0 {
		captureID = resp.PurchaseUnits[0].Payments.Captures[0].ID
	}

	if resp.PaymentSource != nil &&
		resp.PaymentSource.Paypal != nil &&
		resp.PaymentSource.Paypal.Attributes != nil &&
		resp.PaymentSource.Paypal.Attributes.Vault != nil {
		vaultToken = resp.PaymentSource.Paypal.Attributes.Vault.ID
	}
	return
}
