package db

import (
	"database/sql"

	"external_payments/types"
)

func UpsertHMarketUser(u types.HMarketUser) (userID int64, isNew bool, subChanged bool, newSubStatus bool, err error) {
	var existing types.HMarketUser
	var e error

	if u.UniqPhone != nil {
		e = db.Get(&existing, "SELECT * FROM hmarket_users WHERE uniq_phone = ? LIMIT 1", *u.UniqPhone)
	} else if u.Email != "" {
		e = db.Get(&existing, "SELECT * FROM hmarket_users WHERE email = ? LIMIT 1", u.Email)
	} else {
		e = sql.ErrNoRows
	}

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

	res, e := db.Exec(
		`INSERT INTO hmarket_users (first_name, last_name, company, address_1, address_2, city, country, email, phone, uniq_phone, subscribed, blacklisted) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.FirstName, u.LastName, u.Company, u.Address1, u.Address2, u.City, u.Country, u.Email, u.Phone, u.UniqPhone, u.Subscribed, u.Blacklisted,
	)
	if e != nil {
		err = e
		return
	}
	userID, err = res.LastInsertId()
	isNew = true
	return
}

func CreateHMarketActivity(a types.HMarketActivity) error {
	// INSERT IGNORE: silently skips duplicate (cart_token, product_id) for Shopify dedup.
	// NULL cart_token (hw1 activities) is unaffected — NULLs are distinct in unique indexes.
	_, err := db.Exec(
		`INSERT IGNORE INTO hmarket_activities (user_id, source, name, product_id, sku, created_at, cart_token) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.UserID, a.Source, a.Name, a.ProductID, a.SKU, a.CreatedAt, a.CartToken,
	)
	return err
}

func GetHMarketUsers() (users []types.HMarketUser, err error) {
	err = db.Select(&users, `SELECT * FROM hmarket_users ORDER BY id`)
	return
}

func GetHMarketExportData() (rows []types.HMarketExportRow, err error) {
	err = db.Select(&rows, `
		SELECT u.id                      AS user_id,
		       u.first_name,
		       u.last_name,
		       COALESCE(u.phone, '')      AS phone,
		       COALESCE(u.uniq_phone, '') AS uniq_phone,
		       u.email,
		       COALESCE(u.company, '')    AS company,
		       u.city,
		       u.country,
		       u.subscribed,
		       u.blacklisted,
		       a.source,
		       a.name,
		       a.product_id,
		       COALESCE(a.sku, '')        AS sku,
		       a.created_at,
		       COALESCE((
		           SELECT ov.label_en_US
		           FROM civicrm_phone cp
		           JOIN civicrm_value_member_data_223 m ON m.entity_id = cp.contact_id
		           JOIN civicrm_option_value ov ON ov.option_group_id = 606 AND ov.value = m.dropdown_circle_1708
		           WHERE u.uniq_phone IS NOT NULL AND u.uniq_phone != ''
		             AND (REGEXP_REPLACE(cp.phone, '[^0-9]', '') = u.uniq_phone COLLATE utf8mb3_unicode_ci
		                  OR REGEXP_REPLACE(cp.phone, '[^0-9]', '') = CONCAT('0', SUBSTR(u.uniq_phone, 4)) COLLATE utf8mb3_unicode_ci)
		           LIMIT 1
		       ), (
		           SELECT ov.label_en_US
		           FROM civicrm_email ce
		           JOIN civicrm_value_member_data_223 m ON m.entity_id = ce.contact_id
		           JOIN civicrm_option_value ov ON ov.option_group_id = 606 AND ov.value = m.dropdown_circle_1708
		           WHERE ce.email = u.email COLLATE utf8mb3_unicode_ci
		           LIMIT 1
		       ), '') AS circle
		FROM hmarket_users u
		JOIN hmarket_activities a ON a.user_id = u.id
		ORDER BY u.id, a.created_at
	`)
	return
}

func GetHMarketStatus() (users int64, activities int64, err error) {
	err = db.Get(&users, `SELECT COUNT(*) FROM hmarket_users`)
	if err != nil {
		return
	}
	err = db.Get(&activities, `SELECT COUNT(*) FROM hmarket_activities`)
	return
}

func GetHMarketSubHistory() (rows []types.HMarketSubHistoryRecord, err error) {
	err = db.Select(&rows, `
		SELECT id, user_id, description, status, change_type, created_at
		FROM hmarket_subscription_history
		ORDER BY user_id, created_at
	`)
	return
}

func GetHMarketAudiencesByMonth() (rows []types.HMarketAudienceMonthRow, err error) {
	err = db.Select(&rows, `
		SELECT
		  a.source,
		  DATE_FORMAT(a.first_seen, '%Y-%m') AS month,
		  COUNT(DISTINCT a.user_id) AS cnt
		FROM (
		  SELECT user_id, source, MIN(created_at) AS first_seen
		  FROM hmarket_activities
		  GROUP BY user_id, source
		) a
		JOIN hmarket_users u ON u.id = a.user_id AND u.blacklisted = 0
		GROUP BY a.source, DATE_FORMAT(a.first_seen, '%Y-%m')
		ORDER BY a.source, month
	`)
	return
}

// CRMCircleAudienceSource labels the audience row for reported users who belong
// to a circle. It matches the row name used in the audiences sheet.
const CRMCircleAudienceSource = "CRM אנשי קשר בני ברוך"

// unidentifiedCircle is an option in group 606 meaning the circle was never
// identified, so it does not count as belonging to one.
const unidentifiedCircle = "17"

// GetCRMCircleAudienceByMonth counts reported users whose email belongs to a
// CiviCRM contact in a named circle, bucketed by the month the user was first
// seen. The caller accumulates these into a running total.
//
// Users are grouped by user_id alone rather than by (user_id, source), so
// someone active under several sources is counted once — 66 of the reported
// users appear in more than one. Users without an email cannot be matched and
// are excluded.
func GetCRMCircleAudienceByMonth() (rows []types.HMarketAudienceMonthRow, err error) {
	err = db.Select(&rows, `
		SELECT
		  ? AS source,
		  DATE_FORMAT(a.first_seen, '%Y-%m') AS month,
		  COUNT(DISTINCT a.user_id) AS cnt
		FROM (
		  SELECT user_id, MIN(created_at) AS first_seen
		  FROM hmarket_activities
		  GROUP BY user_id
		) a
		JOIN hmarket_users u ON u.id = a.user_id AND u.blacklisted = 0
		WHERE u.email <> ''
		  AND EXISTS (
		    SELECT 1
		    FROM civicrm_email ce
		    JOIN civicrm_value_member_data_223 m ON m.entity_id = ce.contact_id
		    JOIN civicrm_option_value ov ON ov.option_group_id = 606
		         AND ov.value = m.dropdown_circle_1708
		    WHERE ce.email = u.email COLLATE utf8mb3_unicode_ci
		      AND m.dropdown_circle_1708 <> ?
		  )
		GROUP BY month
		ORDER BY month
	`, CRMCircleAudienceSource, unidentifiedCircle)
	return
}

func BlacklistHMarketUser(userID int64, blacklist bool) (found bool, err error) {
	res, e := db.Exec(
		`UPDATE hmarket_users SET blacklisted=? WHERE id=?`,
		blacklist, userID,
	)
	if e != nil {
		return false, e
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func CreateHMarketSubscriptionHistory(h types.HMarketSubscriptionHistory) error {
	_, err := db.Exec(
		`INSERT INTO hmarket_subscription_history (user_id, description, status, change_type) VALUES (?, ?, ?, ?)`,
		h.UserID, h.Description, h.Status, h.ChangeType,
	)
	return err
}
