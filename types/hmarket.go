package types

type HMarketUser struct {
	ID          int64   `db:"id"`
	FirstName   string  `db:"first_name"`
	LastName    string  `db:"last_name"`
	Company     string  `db:"company"`
	Address1    string  `db:"address_1"`
	Address2    string  `db:"address_2"`
	City        string  `db:"city"`
	Country     string  `db:"country"`
	Email       string  `db:"email"`
	Phone       *string `db:"phone"`
	UniqPhone   *string `db:"uniq_phone"`
	Subscribed  bool    `db:"subscribed"`
	Blacklisted bool    `db:"blacklisted"`
}

type HMarketActivity struct {
	UserID    int64  `db:"user_id"`
	Source    string `db:"source"`
	Name      string `db:"name"`
	ProductID int64  `db:"product_id"`
	SKU       string `db:"sku"`
	CreatedAt string `db:"created_at"`
}

type HMarketSubscriptionHistory struct {
	UserID      int64  `db:"user_id"`
	Description string `db:"description"`
	Status      bool   `db:"status"`
}

type HMarketExportRow struct {
	UserID      int64  `db:"user_id"`
	FirstName   string `db:"first_name"`
	LastName    string `db:"last_name"`
	Phone       string `db:"phone"`
	UniqPhone   string `db:"uniq_phone"`
	Email       string `db:"email"`
	Company     string `db:"company"`
	City        string `db:"city"`
	Country     string `db:"country"`
	Subscribed  bool   `db:"subscribed"`
	Blacklisted bool   `db:"blacklisted"`
	Source      string `db:"source"`
	Name        string `db:"name"`
	ProductID   int64  `db:"product_id"`
	SKU         string `db:"sku"`
	CreatedAt   string `db:"created_at"`
}

type HMarketSubHistoryRecord struct {
	ID          int64  `db:"id"`
	UserID      int64  `db:"user_id"`
	Description string `db:"description"`
	Status      bool   `db:"status"`
	CreatedAt   string `db:"created_at"`
}
