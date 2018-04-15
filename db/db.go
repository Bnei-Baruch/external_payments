package db

import (
	_ "github.com/go-sql-driver/mysql"
	"os"
	"log"
	"github.com/gshilin/external_payments/types"
	"github.com/MakeNowJust/heredoc"
	"github.com/jmoiron/sqlx"
	"database/sql"
	"fmt"
)

var (
	db           *sqlx.DB
	storeRequest *sql.Stmt
	//loadRequest       *sql.Stmt
	updateRequestTemp *sql.Stmt
	updateRequest     *sql.Stmt
)

const numOfUpdates = 20

func initDB() (err error) {
	const schema1 = `
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
		details 		VARCHAR(255) NOT NULL,
		participants 	VARCHAR(255) NOT NULL,
		sku			 	VARCHAR(255) NOT NULL,
		vat				VARCHAR(1) NOT NULL,
		installments 	SMALLINT NOT NULL,
		created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		language 		VARCHAR(2) NOT NULL,
		reference 		VARCHAR(10) NOT NULL,
		organization 	TEXT NOT NULL,

		status			VARCHAR(255) NOT NULL DEFAULT 'new',
		UNIQUE (sku, user_key)
	) engine=InnoDB default charset utf8;`
	const schema2 = `
	CREATE TABLE IF NOT EXISTS bb_ext_pelecard_responses (
		user_key	 			VARCHAR(255) NOT NULL,
		pelecard_transaction_id VARCHAR(255),
		pelecard_status_code 	VARCHAR(255),
		approval_no 			VARCHAR(255),
		confirmation_key 		VARCHAR(255),
		param_x 				VARCHAR(255)
	) engine=InnoDB default charset utf8;`
	const schema3 = `
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
	) engine=InnoDB default charset utf8;`
	if _, err = db.Exec(schema1); err != nil {
		log.Fatalf("DB tables 1 creation error: %v\n", err)
	}
	if _, err = db.Exec(schema2); err != nil {
		log.Fatalf("DB tables 2 creation error: %v\n", err)
	}
	if _, err = db.Exec(schema3); err != nil {
		log.Fatalf("DB tables 3 creation error: %v\n", err)
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
		os.Exit(2)
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

	var request string
	request = heredoc.Docf(`
		INSERT INTO bb_ext_requests (
			user_key, good_url, error_url, cancel_url, 
			name, price, currency, email, phone, 
			street, city, country, participants, details, sku, vat, installments, language, 
			reference, organization
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`)
	storeRequest, err = db.Prepare(request)
	if err != nil {
		log.Fatalf("DB storeRequest preparation error: %v\n", err)
		return
	}

	request = heredoc.Docf(`
		INSERT INTO bb_ext_pelecard_responses (
			user_key, pelecard_transaction_id, pelecard_status_code, approval_no, confirmation_key, param_x
		) VALUES (
			?, ?, ?, ?, ?, ?
		)
	`)
	updateRequestTemp, err = db.Prepare(request)
	if err != nil {
		log.Fatalf("DB updateRequestTemp preparation error: %v\n", err)
		return
	}

	request = heredoc.Docf(`
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
	updateRequest, err = db.Prepare(request)
	if err != nil {
		log.Fatalf("DB updateRequest preparation error: %v\n", err)
		return
	}

	return
}

func Disconnect() {
	storeRequest.Close()
	updateRequestTemp.Close()
	updateRequest.Close()
	db.Close()
}

func StoreRequest(p types.PaymentRequest) (lastId int64, err error) {
	var result sql.Result
	result, err = storeRequest.Exec(
		p.UserKey, p.GoodURL, p.ErrorURL, p.CancelURL,
		p.Name, p.Price, p.Currency, p.Email, p.Phone, p.Street, p.City, p.Country,
		p.Participans, p.Details, p.SKU, p.VAT, p.Installments, p.Language, p.Reference, p.Organization);
	if err != nil {
		fmt.Printf("DB StoreRequest Error: %v\n", err)
		return
	}
	lastId, err = result.LastInsertId()
	if err != nil {
		fmt.Printf("DB StoreRequest LastInsertId Error: %v\n", err)
		return
	}
	return
}

func LoadRequest(userKey string, p *types.PaymentRequest) (err error) {
	err = db.Get(p, "SELECT * FROM bb_ext_requests WHERE user_key = $1 LIMIT 1", userKey)
	return
}

func UpdateRequestTemp(userKey string, p types.PeleCardResponse) (err error) {
	_, err = updateRequestTemp.Exec(
		userKey,
		p.PelecardTransactionId, p.PelecardStatusCode, p.ApprovalNo, p.ConfirmationKey, p.ParamX)
	if err != nil {
		fmt.Printf("DB UpdateRequestTemp Request Error: %v\n", err)
	}
	return
}

func UpdateRequest(p types.PaymentResponse) (err error) {
	_, err = updateRequest.Exec(
		p.UserKey,
		p.TransactionId, p.CardHebrewName, p.TransactionUpdateTime, p.CreditCardAbroadCard,
		p.FirstPaymentTotal, p.CreditType, p.CreditCardBrand, p.VoucherId, p.StationNumber,
		p.AdditionalDetailsParamX, p.CreditCardCompanyIssuer, p.DebitCode, p.FixedPaymentTotal,
		p.CreditCardNumber, p.CreditCardExpDate, p.CreditCardCompanyClearer,
		p.DebitTotal, p.TotalPayments, p.DebitType, p.TransactionInitTime, p.JParam,
		p.TransactionPelecardId, p.DebitCurrency)
	if err != nil {
		fmt.Printf("DB UpdateRequest Error: %v\n", err)
	}
	return
}
