package hmarket

import "strings"

// maskEmail keeps the first character and the domain, so a mismatch between a
// webhook and a stored user is still recognisable without writing the address
// down: "shopper@gmail.com" -> "s***@gmail.com".
func maskEmail(s string) string {
	if s == "" {
		return ""
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 {
		return "***"
	}
	return s[:1] + "***" + s[at:]
}

// maskPhone keeps the last four digits, enough to match against a user record
// by eye. Formatting is dropped along with the rest.
func maskPhone(s string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if digits == "" {
		return ""
	}
	if len(digits) <= 4 {
		return "***"
	}
	return "***" + digits[len(digits)-4:]
}
