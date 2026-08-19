package types

import (
	"fmt"
	"strings"
	"testing"
)

func TestPaymentRequestStringOmitsSensitiveFields(t *testing.T) {
	r := PaymentRequest{
		UserKey:      "ord-123",
		Reference:    "m-456",
		Organization: "ben2",
		Price:        180,
		Currency:     "ILS",
		SKU:          "donation",
		Token:        "9876543210123456",
		Name:         "Israel Israeli",
		Email:        "donor@example.com",
		Phone:        "+972500000000",
		Street:       "Hertzl 1",
		City:         "Petah Tikva",
		TaxId:        "123456789",
		ApprovalNo:   "0012345",
	}

	out := fmt.Sprintf("%+v", r)

	for _, secret := range []string{
		r.Token, r.Name, r.Email, r.Phone, r.Street, r.City, r.TaxId,
	} {
		if strings.Contains(out, secret) {
			t.Errorf("PaymentRequest log output leaks %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "ord-123") || !strings.Contains(out, "m-456") {
		t.Errorf("PaymentRequest log output lost its correlation keys: %s", out)
	}
	if !strings.Contains(out, "3456") {
		t.Errorf("PaymentRequest log output should keep the token suffix: %s", out)
	}
}

func TestPeleCardResponseStringOmitsSensitiveFields(t *testing.T) {
	f := PeleCardResponse{
		UserKey:               "ord-123",
		ParamX:                "m-456",
		PelecardTransactionId: "tx-789",
		PelecardStatusCode:    "000",
		Token:                 "9876543210123456",
		ConfirmationKey:       "conf-secret",
		ApprovalNo:            "0012345",
	}

	out := fmt.Sprintf("%+v", f)

	for _, secret := range []string{f.Token, f.ConfirmationKey, f.ApprovalNo} {
		if strings.Contains(out, secret) {
			t.Errorf("PeleCardResponse log output leaks %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "tx-789") {
		t.Errorf("PeleCardResponse log output lost the transaction id: %s", out)
	}
}

func TestLast4(t *testing.T) {
	for in, want := range map[string]string{
		"":                 "",
		"12":               "****",
		"1234":             "****",
		"9876543210123456": "***3456",
	} {
		if got := last4(in); got != want {
			t.Errorf("last4(%q) = %q, want %q", in, got, want)
		}
	}
}
