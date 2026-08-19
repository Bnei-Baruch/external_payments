package types

import "fmt"

// last4 keeps only the trailing characters of a sensitive value, so a log line
// can still be correlated with the gateway without carrying anything reusable.
func last4(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "****"
	}
	return "***" + s[len(s)-4:]
}

// String redacts the card token and personal data (name, email, phone, address,
// tax id) from PaymentRequest. fmt calls this for %v and %+v, so every existing
// log site that prints a request is covered, and new ones are safe by default.
func (p PaymentRequest) String() string {
	return fmt.Sprintf("{UserKey:%s Reference:%s Organization:%s Price:%.2f %s "+
		"SKU:%s Installments:%d Recurring:%t Status:%s PStatus:%s Token:%s}",
		p.UserKey, p.Reference, p.Organization, p.Price, p.Currency,
		p.SKU, p.Installments, p.IsRecurring, p.Status, p.PStatus,
		last4(p.Token))
}

// String redacts the token, confirmation key and approval number from the
// gateway callback form.
func (p PeleCardResponse) String() string {
	return fmt.Sprintf("{UserKey:%s ParamX:%s TransactionId:%s StatusCode:%s Token:%s}",
		p.UserKey, p.ParamX, p.PelecardTransactionId, p.PelecardStatusCode,
		last4(p.Token))
}
