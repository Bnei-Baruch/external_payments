package db

import (
	"github.com/MakeNowJust/heredoc"

	"external_payments/types"
)

// SetPaypalOrderId stores the PayPal order ID and environment so pp2fix can reconcile stuck payments.
func SetPaypalOrderId(userKey, orderID, env string) error {
	return execInTx(
		"UPDATE civicrm_bb_ext_requests SET paypal_order_id = ?, paypal_env = ? WHERE user_key = ? ORDER BY id DESC LIMIT 1",
		orderID, env, userKey,
	)
}

// StorePaypalCapture inserts a captured PayPal payment into civicrm_bb_ext_paypal
// with status='new' so pp2prio forwards it to Priority ERP.
func StorePaypalCapture(req types.PaymentRequest, captureID, paymentDate, env string) error {
	query := heredoc.Doc(`
		INSERT INTO civicrm_bb_ext_paypal (
			name, price, currency, email, phone, street, city, country, details, sku, language,
			reference, organization, transaction_id, payment_date, voucher_id, invoice, paypal_env
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', ?
		)
	`)
	return execInTx(query,
		req.Name, req.Price, req.Currency, req.Email, req.Phone,
		req.Street, req.City, req.Country, req.Details, req.SKU,
		req.Language, req.Reference, req.Organization,
		captureID, paymentDate, env,
	)
}
