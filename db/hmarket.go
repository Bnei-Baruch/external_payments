package db

import (
	"database/sql"

	"external_payments/types"
)

func UpsertHMarketUser(u types.HMarketUser) (userID int64, subChanged bool, newSubStatus bool, err error) {
	if u.UniqPhone != nil {
		var existing types.HMarketUser
		e := db.Get(&existing, "SELECT * FROM hmarket_users WHERE uniq_phone = ? LIMIT 1", *u.UniqPhone)
		if e == nil {
			u.Blacklisted = existing.Blacklisted
			subChanged = existing.Subscribed != u.Subscribed
			newSubStatus = u.Subscribed
			_, err = db.Exec(
				`UPDATE hmarket_users SET first_name=?, last_name=?, company=?, address_1=?, address_2=?, city=?, country=?, email=?, subscribed=?, blacklisted=? WHERE id=?`,
				u.FirstName, u.LastName, u.Company, u.Address1, u.Address2, u.City, u.Country, u.Email, u.Subscribed, u.Blacklisted, existing.ID,
			)
			userID = existing.ID
			return
		}
		if e != sql.ErrNoRows {
			err = e
			return
		}
	}

	res, e := db.Exec(
		`INSERT INTO hmarket_users (first_name, last_name, company, address_1, address_2, city, country, email, phone, uniq_phone, subscribed, blacklisted) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.FirstName, u.LastName, u.Company, u.Address1, u.Address2, u.City, u.Country, u.Email, u.Phone, u.UniqPhone, u.Subscribed, u.Blacklisted,
	)
	if e != nil {
		err = e
		return
	}
	userID, err = res.LastInsertId()
	return
}

func CreateHMarketActivity(a types.HMarketActivity) error {
	_, err := db.Exec(
		`INSERT INTO hmarket_activities (user_id, source, name, product_id, sku, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		a.UserID, a.Source, a.Name, a.ProductID, a.SKU, a.CreatedAt,
	)
	return err
}

func CreateHMarketSubscriptionHistory(h types.HMarketSubscriptionHistory) error {
	_, err := db.Exec(
		`INSERT INTO hmarket_subscription_history (user_id, description, status) VALUES (?, ?, ?)`,
		h.UserID, h.Description, h.Status,
	)
	return err
}
