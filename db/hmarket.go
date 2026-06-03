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

func GetHMarketUsers() (users []types.HMarketUser, err error) {
	err = db.Select(&users, `SELECT * FROM hmarket_users ORDER BY id`)
	return
}

func GetHMarketExportData() (rows []types.HMarketExportRow, err error) {
	err = db.Select(&rows, `
		SELECT u.id              AS user_id,
		       u.first_name,
		       u.last_name,
		       COALESCE(u.phone, '') AS phone,
		       u.email,
		       COALESCE(u.company, '') AS company,
		       u.city,
		       u.country,
		       a.source,
		       a.name,
		       a.product_id,
		       COALESCE(a.sku, '') AS sku,
		       a.created_at
		FROM hmarket_users u
		JOIN hmarket_activities a ON a.user_id = u.id
		ORDER BY u.id, a.created_at
	`)
	return
}

func GetHMarketSubHistory() (rows []types.HMarketSubHistoryRecord, err error) {
	err = db.Select(&rows, `
		SELECT id, user_id, description, status, created_at
		FROM hmarket_subscription_history
		ORDER BY user_id, created_at
	`)
	return
}

func CreateHMarketSubscriptionHistory(h types.HMarketSubscriptionHistory) error {
	_, err := db.Exec(
		`INSERT INTO hmarket_subscription_history (user_id, description, status) VALUES (?, ?, ?)`,
		h.UserID, h.Description, h.Status,
	)
	return err
}
