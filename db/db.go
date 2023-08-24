package db

import (
	"fmt"
	"log"
	"os"

	"github.com/MakeNowJust/heredoc"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"

	"external_payments/types"
)

var (
	db *sqlx.DB
)

const numOfUpdates = 20

func initDB() (err error) {
	schemas := []string{
		heredoc.Doc(`
	CREATE TABLE IF NOT EXISTS bb_ext_paypal (
		id           	BIGINT PRIMARY KEY AUTO_INCREMENT,
		
		name 			VARCHAR(255) NOT NULL,
		price 			REAL NOT NULL,
		currency 		VARCHAR(255) NOT NULL,
		email 			VARCHAR(255) NOT NULL,
		phone 			VARCHAR(255) NOT NULL,
		street 			VARCHAR(255) NOT NULL,
		city 			VARCHAR(255) NOT NULL,
		country 		VARCHAR(255) NOT NULL,
		details 		TEXT NOT NULL,
		sku			 	VARCHAR(255) NOT NULL,
		language 		VARCHAR(2) NOT NULL,
		reference 		VARCHAR(20) NOT NULL,
		organization 	TEXT NOT NULL,
		transaction_id 	VARCHAR(255),
		payment_date 	VARCHAR(255),
		voucher_id 		VARCHAR(255),
		invoice 		VARCHAR(255),

		status			VARCHAR(255) NOT NULL DEFAULT 'new',

		created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	) engine=InnoDB default charset utf8;`),
		heredoc.Doc(`
	CREATE TABLE IF NOT EXISTS bb_ext_requests (
		id           	BIGINT PRIMARY KEY AUTO_INCREMENT,
		
		user_key	 	VARCHAR(255) NOT NULL,
		
		good_url	 	TEXT NOT NULL,
		error_url	 	TEXT NOT NULL,
		cancel_url	 	TEXT NOT NULL,

		name 			VARCHAR(255) NOT NULL,
		price 			REAL NOT NULL,
		currency 		VARCHAR(255) NOT NULL,
		email 			VARCHAR(255) NOT NULL,
		phone 			VARCHAR(255) NOT NULL,
		street 			VARCHAR(255) NOT NULL,
		city 			VARCHAR(255) NOT NULL,
		country 		VARCHAR(255) NOT NULL,
		details 		TEXT NOT NULL,
		participants 	VARCHAR(255) NOT NULL,
		sku			 	VARCHAR(255) NOT NULL,
		vat				VARCHAR(1) NOT NULL,
		installments 	SMALLINT NOT NULL,
		created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		language 		VARCHAR(2) NOT NULL,
		reference 		VARCHAR(20) NOT NULL,
		organization 	TEXT NOT NULL,
		is_visual       TINYINT(1),

		status			VARCHAR(255) NOT NULL DEFAULT 'new'
	) engine=InnoDB default charset utf8;`),
		heredoc.Doc(`
	CREATE TABLE IF NOT EXISTS bb_ext_pelecard_responses (
		user_key	 			VARCHAR(255) NOT NULL,
		pelecard_transaction_id VARCHAR(255),
		pelecard_status_code 	VARCHAR(255),
		confirmation_key 		VARCHAR(255),
		param_x 				VARCHAR(255)
	) engine=InnoDB default charset utf8;`),
		heredoc.Doc(`
	CREATE TABLE IF NOT EXISTS bb_ext_payment_responses (
		user_key	 				VARCHAR(255) NOT NULL,
		transaction_id 				VARCHAR(255),
		card_hebrew_name 			VARCHAR(255),
		transaction_update_time 	VARCHAR(255),
		credit_card_abroad_card 	VARCHAR(255),
		first_payment_total 		VARCHAR(255),
		credit_type 				VARCHAR(255),
		credit_card_brand 			VARCHAR(255),
		voucher_id 					VARCHAR(255),
		station_number 				VARCHAR(255),
		additional_details_param_x 	VARCHAR(255),
		credit_card_company_issuer 	VARCHAR(255),
		debit_code 					VARCHAR(255),
		fixed_payment_total 		VARCHAR(255),
		credit_card_number 			VARCHAR(255),
		credit_card_exp_date 		VARCHAR(255),
		credit_card_company_clearer VARCHAR(255),
		debit_total 				VARCHAR(255),
		total_payments 				VARCHAR(255),
		debit_type 					VARCHAR(255),
		transaction_init_time 		VARCHAR(255),
		j_param 					VARCHAR(255),
		transaction_pelecard_id 	VARCHAR(255),
		debit_currency 				VARCHAR(255)
	) engine=InnoDB default charset utf8;`),
	}
	for idx, schema := range schemas {
		if _, err = db.Exec(schema); err != nil {
			log.Fatalf("DB tables %d creation error: %v\n", idx, err)
		}
	}
	return
}

func Connect() (err error) {
	host := os.Getenv("CIVI_HOST")
	if host == "" {
		host = "localhost"
	}
	dbName := os.Getenv("CIVI_DBNAME")
	if dbName == "" {
		dbName = "localhost"
	}
	user := os.Getenv("CIVI_USER")
	if user == "" {
		log.Fatalf("Unable to connect without username\n")
	}
	password := os.Getenv("CIVI_PASSWORD")
	if password == "" {
		log.Fatalf("Unable to connect without password\n")
	}
	protocol := os.Getenv("CIVI_PROTOCOL")
	if protocol == "" {
		log.Fatalf("Unable to connect without protocol\n")
	}

	dsn := fmt.Sprintf("%s:%s@%s(%s)/%s", user, password, protocol, host, dbName)
	if db, err = sqlx.Open("mysql", dsn); err != nil {
		log.Fatalf("DB connection error: %v\n", err)
		return
	}
	err = db.Ping() // really connect to db
	if err != nil {
		log.Fatalf("DB real connection error: %v\n", err)
		return
	}

	db.SetMaxOpenConns(numOfUpdates)
	db.SetMaxIdleConns(numOfUpdates)

	if err = initDB(); err != nil {
		log.Fatalf("DB initialization error: %v\n", err)
		return
	}

	return
}

func Disconnect() {
	_ = db.Close()
}

func StorePaypal(p types.PaypalRegister) {
	request := heredoc.Doc(`
		INSERT INTO bb_ext_paypal (
			name, price, currency, email, phone, street, city, country, details, sku, language, 
			reference, organization, transaction_id, payment_date, voucher_id, invoice
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`)
	err := execInTx(request,
		p.Name, p.Price, p.Currency, p.Email, p.Phone, p.Street, p.City, p.Country,
		p.Details, p.SKU, p.Language, p.Reference, p.Organization,
		p.TransactionId, p.PaymentDate, p.VoucherId, p.Invoice,
	)

	if err != nil {
		fmt.Print("\n", err)
	}

	return
}

func StoreRequest(p types.PaymentRequest) (err error) {
	request := heredoc.Doc(`
		INSERT INTO bb_ext_requests (
			user_key, good_url, error_url, cancel_url, 
			name, price, currency, email, phone, 
			street, city, country, participants, details, sku, vat, installments, language, 
			reference, organization, is_visual
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`)

	err = execInTx(request,
		p.UserKey, p.GoodURL, p.ErrorURL, p.CancelURL,
		p.Name, p.Price, p.Currency, p.Email, p.Phone, p.Street, p.City, p.Country,
		p.Participans, p.Details, p.SKU, p.VAT, p.Installments, p.Language, p.Reference,
		p.Organization, p.IsVisual,
	)
	return
}

func SetStatus(userKey string, value string) {
	request := heredoc.Doc(`
		UPDATE bb_ext_requests SET status = ?, pstatus = ? 
		WHERE user_key = ?
		ORDER BY id DESC
		LIMIT 1
	`)

	_ = execInTx(request, value, value, userKey)
}

func LoadRequest(userKey string, p *types.PaymentRequest) (err error) {
	err = db.Get(p, "SELECT * FROM bb_ext_requests WHERE user_key = ? ORDER BY id DESC LIMIT 1", userKey)
	return
}

func GetOrganization(userKey string) (org string, err error) {
	err = db.Get(&org,
		heredoc.Doc(`
			SELECT organization 
			FROM bb_ext_requests 
			WHERE user_key = ?
            ORDER BY id DESC
			LIMIT 1
		`), userKey)
	return
}

func Confirm(p *types.ConfirmRequest) bool {
	request := types.PaymentRequest{}
	err := db.Get(&request,
		heredoc.Doc(`
			SELECT * 
			FROM bb_ext_requests 
			WHERE status = 'valid' AND user_key = ? AND price = ? 
				  AND currency = ? AND sku = ? AND reference = ? 
				  AND organization = ?
            ORDER BY id DESC
			LIMIT 1
		`),
		p.UserKey, p.Price, p.Currency, p.SKU, p.Reference, p.Organization)
	return err == nil
}

func UpdateRequestTemp(userKey string, p types.PeleCardResponse) (err error) {
	request := heredoc.Doc(`
		INSERT INTO bb_ext_pelecard_responses (
			user_key, pelecard_transaction_id, pelecard_status_code, confirmation_key, param_x
		) VALUES (
			?, ?, ?, ?, ?
		)
	`)

	err = execInTx(request,
		userKey,
		p.PelecardTransactionId, p.PelecardStatusCode, p.ConfirmationKey, p.ParamX)
	return
}

func UpdateRequest(p types.PaymentResponse) (err error) {
	request := heredoc.Doc(`
		INSERT INTO bb_ext_payment_responses (
			user_key,
			transaction_id, card_hebrew_name, transaction_update_time, credit_card_abroad_card,
			first_payment_total, credit_type, credit_card_brand, voucher_id, station_number,
			additional_details_param_x, credit_card_company_issuer, debit_code, fixed_payment_total,
			credit_card_number, credit_card_exp_date, credit_card_company_clearer, debit_total,
			total_payments, debit_type, transaction_init_time, j_param, transaction_pelecard_id,
			debit_currency
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`)

	err = execInTx(request,
		p.UserKey,
		p.TransactionId, p.CardHebrewName, p.TransactionUpdateTime, p.CreditCardAbroadCard,
		p.FirstPaymentTotal, p.CreditType, p.CreditCardBrand, p.VoucherId, p.StationNumber,
		p.AdditionalDetailsParamX, p.CreditCardCompanyIssuer, p.DebitCode, p.FixedPaymentTotal,
		p.CreditCardNumber, p.CreditCardExpDate, p.CreditCardCompanyClearer,
		p.DebitTotal, p.TotalPayments, p.DebitType, p.TransactionInitTime, p.JParam,
		p.TransactionPelecardId, p.DebitCurrency)
	return
}

func execInTx(query string, args ...interface{}) (err error) {
	var er error
	tx := db.MustBegin()
	_, err = tx.Exec(query, args...)
	if err != nil {
		er = tx.Rollback()
		if er != nil {
			fmt.Println("Query:", query, "\nParams:", args)
			fmt.Println("Query error:", err)
			fmt.Println("Rollback error:", er)
		}
	} else {
		er = tx.Commit()
		if er != nil {
			fmt.Println("Query:", query, "\nParams:", args)
			fmt.Println("Query error:", err)
			fmt.Println("Commit error:", er)
		}
	}
	return
}

func GetProject(projectName string) (project types.Project, err error) {
	err = db.Get(&project,
		heredoc.Doc(`
			SELECT *
			FROM bb_projects 
			WHERE name = ?
			LIMIT 1
		`), projectName)
	return
}

func GetProjectTotals(projectName string, currency string, date string) (project types.ProjectTotals, err error) {
	err = db.Get(&project,
		heredoc.Doc(`
SELECT count(1) AS contributors, sum(co.total_amount) AS total
FROM civicrm_contribution co
INNER JOIN civicrm_value_maser_55 pr ON pr.entity_id = co.id AND pr.event_for_activity_1417 = ?
WHERE (co.contribution_status_id = 1) 
  AND (co.financial_type_id = 21)
  AND (co.receive_date >= ?)
  AND (co.currency = ?);
		`), projectName, date, currency)
	return
}

func GetProjectRanges(projectName string, date string) (prange []types.ProjectRange, err error) {
	err = db.Select(&prange,
		heredoc.Doc(`
WITH a AS (
	SELECT IF(co.currency = 'USD', total_amount, IF(co.currency = 'EUR', total_amount * 1.1, total_amount / 4.13)) AS amount
	FROM civicrm_contribution co
	INNER JOIN civicrm_value_maser_55 pr ON pr.entity_id = co.id AND pr.event_for_activity_1417 = ?
	WHERE (co.contribution_status_id = 1) AND (co.financial_type_id = 21) AND (co.receive_date >= ?)
)
SELECT 1 AS start, 9 AS finish, COALESCE(sum(1), 0) AS contributors FROM a WHERE a.amount BETWEEN 1 AND 9
UNION
SELECT 10 AS start, 99 AS finish, COALESCE(sum(1), 0) AS contributors FROM a WHERE a.amount BETWEEN 10 AND 99
UNION
SELECT 100 AS start, 999 AS finish, COALESCE(sum(1), 0) AS contributors FROM a WHERE a.amount BETWEEN 100 AND 999
UNION
SELECT 1000 AS start, 4999 AS finish, COALESCE(sum(1), 0) AS contributors FROM a WHERE a.amount BETWEEN 1000 AND 4999
UNION
SELECT 5000 AS start, 9999 AS finish, COALESCE(sum(1), 0) AS contributors FROM a WHERE a.amount BETWEEN 5000 AND 9999
UNION
SELECT 10000 AS start, 99999 AS finish, COALESCE(sum(1), 0) AS contributors FROM a WHERE a.amount BETWEEN 10000 AND 99999
UNION
SELECT 100000 AS start, 999999 AS finish, COALESCE(sum(1), 0) AS contributors FROM a WHERE a.amount BETWEEN 100000 AND 999999
UNION
SELECT 1000000 AS start, 9999999 AS finish, COALESCE(sum(1), 0) AS contributors FROM a WHERE a.amount BETWEEN 1000000 AND 9999999
		`), projectName, date)
	return
}

func GetProjectByCountry(projectName string, date string) (byCountry []types.ProjectByCountry, err error) {
	err = db.Select(&byCountry,
		heredoc.Doc(`
WITH a AS (
	SELECT country.name AS country,
		IF(co.currency = 'USD', total_amount, IF(co.currency = 'EUR', total_amount * 1.1, total_amount / 4.13)) AS amount
	FROM civicrm_contribution co
	INNER JOIN civicrm_value_maser_55 pr ON pr.entity_id = co.id AND pr.event_for_activity_1417 = ?
	LEFT JOIN civicrm_address address
		ON co.contact_id = address.contact_id AND address.location_type_id = 5 AND address.is_primary = 1
	LEFT JOIN civicrm_country country ON address.country_id = country.id
	WHERE co.contribution_status_id = 1 AND co.financial_type_id = 21 AND co.receive_date >= ?
)
SELECT country, sum(amount) as sum, count(1) contributors
FROM a
GROUP BY country
		`), projectName, date)
	return
}
