package db

import (
	_ "github.com/lib/pq"
	"os"
	"log"
	"github.com/gshilin/external_payments/types"
	"github.com/MakeNowJust/heredoc"
	"github.com/jmoiron/sqlx"
	"database/sql"
	"fmt"
)

var (
	db                *sqlx.DB
	storeRequest      *sql.Stmt
	loadRequest       *sql.Stmt
	updateRequestTemp *sql.Stmt
	updateRequest     *sql.Stmt
)

const numOfUpdates = 20

func initDB() (err error) {
	const schema = `
	CREATE TABLE IF NOT EXISTS requests (
		id           	BIGSERIAL PRIMARY KEY,

		user_key	 	VARCHAR(255) NOT NULL,
		good_url	 	TEXT NOT NULL,
		error_url	 	TEXT NOT NULL,
		cancel_url	 	TEXT NOT NULL,

		organization 	TEXT NOT NULL,
		sku			 	VARCHAR(255) NOT NULL ,
		vat				BOOLEAN NOT NULL ,
		name 			VARCHAR(255) NOT NULL ,
		details 		VARCHAR(255) NOT NULL ,
		price 			REAL NOT NULL ,
		currency 		VARCHAR(255) NOT NULL ,
		installments 	SMALLINT NOT NULL ,
		email 			VARCHAR(255) NOT NULL ,
		street 			VARCHAR(255) NOT NULL ,
		city 			VARCHAR(255) NOT NULL ,
		country 		VARCHAR(255) NOT NULL ,
		language 		VARCHAR(2) NOT NULL,

		UNIQUE (sku, user_key)
	);
	CREATE TABLE IF NOT EXISTS pelecard_responses (
		user_key	 			VARCHAR(255) NOT NULL,
		pelecard_transaction_id VARCHAR(255) ,
		pelecard_status_code 	VARCHAR(255) ,
		approval_no 			VARCHAR(255) ,
		confirmation_key 		VARCHAR(255) ,
		param_x 				VARCHAR(255)
	);
	CREATE TABLE IF NOT EXISTS payment_responses (
		user_key	 				VARCHAR(255) NOT NULL,
		transaction_id 				VARCHAR(255) ,
		card_hebrew_name 			VARCHAR(255) ,
		transaction_update_time 	VARCHAR(255) ,
		credit_card_abroad_card 	VARCHAR(255) ,
		first_payment_total 		VARCHAR(255) ,
		credit_type 				VARCHAR(255) ,
		credit_card_brand 			VARCHAR(255) ,
		voucher_id 					VARCHAR(255) ,
		station_number 				VARCHAR(255) ,
		additional_details_param_x 	VARCHAR(255) ,
		credit_card_company_issuer 	VARCHAR(255) ,
		debit_code 					VARCHAR(255) ,
		fixed_payment_total 		VARCHAR(255) ,
		credit_card_number 			VARCHAR(255) ,
		credit_card_exp_date 		VARCHAR(255) ,
		credit_card_company_clearer VARCHAR(255) ,
		debit_total 				VARCHAR(255) ,
		total_payments 				VARCHAR(255) ,
		debit_type 					VARCHAR(255) ,
		transaction_init_time 		VARCHAR(255) ,
		j_param 					VARCHAR(255) ,
		transaction_pelecard_id 	VARCHAR(255) ,
		debit_currency 				VARCHAR(255)
	);
	`

	if _, err = db.Exec(schema); err != nil {
		log.Fatalf("DB tables creation error: %v\n", err)
	}
	return
}

func Connect() (err error) {
	db, err = sqlx.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
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
		INSERT INTO requests (
			user_key, good_url, error_url, cancel_url, organization, sku, vat, name,
			details, price, currency, installments, email, street, city, country, language
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		) RETURNING id
	`)
	storeRequest, err = db.Prepare(request)
	if err != nil {
		log.Fatalf("DB storeRequest preparation error: %v\n", err)
		return
	}

	request = heredoc.Docf(`
		INSERT INTO pelecard_responses (
			user_key, pelecard_transaction_id, pelecard_status_code, approval_no, confirmation_key, param_x
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`)
	updateRequestTemp, err = db.Prepare(request)
	if err != nil {
		log.Fatalf("DB updateRequestTemp preparation error: %v\n", err)
		return
	}

	request = heredoc.Docf(`
		INSERT INTO payment_responses (
			user_key,
			transaction_id, card_hebrew_name, transaction_update_time, credit_card_abroad_card,
			first_payment_total, credit_type, credit_card_brand, voucher_id, station_number,
			additional_details_param_x, credit_card_company_issuer, debit_code, fixed_payment_total,
			credit_card_number, credit_card_exp_date, credit_card_company_clearer, debit_total,
			total_payments, debit_type, transaction_init_time, j_param, transaction_pelecard_id,
			debit_currency
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24
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
	db.Close()
}

func StoreRequest(p types.PaymentRequest) (lastId int64, err error) {
	err = storeRequest.QueryRow(
		p.UserKey, p.GoodURL, p.ErrorURL, p.CancelURL, p.Org, p.SKU, p.VAT, p.Name,
		p.Details, p.Price, p.Currency, p.Installments, p.Email, p.Street, p.City, p.Country, p.Language).Scan(&lastId)
	if err != nil {
		fmt.Printf("DB StoreRequest Error: %v\n", err)
	}
	return
}

func LoadRequest(userKey string, p *types.PaymentRequest) (err error) {
	//udb := db.Unsafe()
	err = db.Get(p, "SELECT * FROM requests WHERE user_key = $1 LIMIT 1", userKey)
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
