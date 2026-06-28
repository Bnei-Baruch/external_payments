package db

import (
	"github.com/MakeNowJust/heredoc"

	"external_payments/types"
)

// SetPaypalOrderId stores the PayPal order ID so pp2fix can reconcile stuck payments.
func SetPaypalOrderId(userKey, orderID string) error {
	return execInTx(
		"UPDATE civicrm_bb_ext_requests SET paypal_order_id = ? WHERE user_key = ? ORDER BY id DESC LIMIT 1",
		orderID, userKey,
	)
}

// StorePaypalCapture inserts a captured PayPal payment into civicrm_bb_ext_paypal
// with status='new' so pp2prio forwards it to Priority ERP.
func StorePaypalCapture(req types.PaymentRequest, captureID string, paymentDate string) error {
	query := heredoc.Doc(`
		INSERT INTO civicrm_bb_ext_paypal (
			name, price, currency, email, phone, street, city, country, details, sku, language,
			reference, organization, transaction_id, payment_date, voucher_id, invoice
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ''
		)
	`)
	return execInTx(query,
		req.Name, req.Price, req.Currency, req.Email, req.Phone,
		req.Street, req.City, req.Country, req.Details, req.SKU,
		req.Language, req.Reference, req.Organization,
		captureID, paymentDate,
	)
}
