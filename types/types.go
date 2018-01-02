package types

type PaymentRequest struct {
	Id uint64 `json:"-" sql:"id,omitempty"`

	// Part for Pelecard
	UserKey   string `json:"userKey" form:"userKey" db:"user_key" validate:"string,required"`
	GoodURL   string `json:"goodURL" form:"goodURL" db:"good_url" validate:"string,required"`
	ErrorURL  string `json:"errorURL" form:"errorURL" db:"error_url" validate:"string,required"`
	CancelURL string `json:"cancelURL" form:"cancelURL" db:"cancel_url" validate:"string,required"`

	// Part for Priority
	Org          string  `json:"org" form:"org" db:"organization" validate:"string,required"`
	SKU          string  `json:"sku" form:"sku" db:"sku" validate:"string,required"`
	VAT          string  `json:"vat" form:"vat" db:"vat" validate:"bool,required,values=y|Y|n|N|t|T|f|F"`
	Name         string  `json:"name" form:"name" db:"name" validate:"string,required"`
	Details      string  `json:"details" form:"details" db:"details" validate:"string"`
	Price        float64 `json:"price" form:"price" db:"price" validate:"float"`
	Currency     string  `json:"currency" form:"currency" db:"currency" validate:"string,required,values=USD|EUR|NIS"`
	Installments int     `json:"installments" form:"installments" db:"installments" validate:"number,min=1,max=12"`
	Email        string  `json:"email" form:"email" db:"email" validate:"email,required"`
	Street       string  `json:"street" form:"street" db:"street" validate:"string,required"`
	City         string  `json:"city" form:"city" db:"city" validate:"string,required"`
	Country      string  `json:"country" form:"country" db:"country" validate:"string,required"`
	Language     string  `json:"language" form:"language" db:"language" validate:"string,required,values=EN|HE"`

	PelecardTransactionId string `db:"pelecard_transaction_id"`
	PelecardStatusCode string `db:"pelecard_status_code"`
	ApprovalNo string `db:"approval_no"`
	ConfirmationKey string `db:"confirmation_key"`
	ParamX string `db:"param_x"`

	TransactionId string `db:"transaction_id"`
	CardHebrewName string `db:"card_hebrew_name"`
	TransactionUpdateTime string `db:"transaction_update_time"`
	CreditCardAbroadCard string `db:"credit_card_abroad_card"`
	FirstPaymentTotal string `db:"first_payment_total"`
	CreditType string `db:"credit_type"`
	CreditCardBrand string `db:"credit_card_brand"`
	VoucherId string `db:"voucher_id"`
	StationNumber string `db:"station_number"`
	AdditionalDetailsParamX string `db:"additional_details_param_x"`
	CreditCardCompanyIssuer string `db:"credit_card_company_issuer"`
	DebitCode string `db:"debit_code"`
	FixedPaymentTotal string `db:"fixed_payment_total"`
	CreditCardNumber string `db:"credit_card_number"`
	CreditCardExpDate string `db:"credit_card_exp_date"`
	CreditCardCompanyClearer string `db:"credit_card_company_clearer"`
	DebitTotal string `db:"debit_total"`
	TotalPayment string `db:"total_payments"`
	DebitType string `db:"debit_type"`
	TransactionInitTime string `db:"transaction_init_time"`
	JParam string `db:"j_param"`
	TransactionPelecardId string `db:"transaction_pelecard_id"`
	DebitCurrenct string `db:"debit_currency"`
}

type PeleCardResponse struct {
	UserKey               string `db:"user_key"`
	PelecardTransactionId string `db:"pelecard_transaction_id"`
	PelecardStatusCode    string `db:"pelecard_status_code"`
	ConfirmationKey       string `db:"confirmation_key"`
	ApprovalNo            string `db:"approval_no"`
	ParamX                string `db:"param_x"`
}

type PaymentResponse struct {
	UserKey               string `db:"user_key" url:"user_key"`
	TransactionId            string `db:"transaction_id" url:"transaction_id"`
	CardHebrewName           string `db:"card_hebrew_name" url:"card_hebrew_name"`
	TransactionUpdateTime    string `db:"transaction_update_time" url:"transaction_update_time"`
	CreditCardAbroadCard     string `db:"credit_card_abroad_card" url:"credit_card_abroad_card"`
	FirstPaymentTotal        string `db:"first_payment_total" url:"first_payment_total"`
	CreditType               string `db:"credit_type" url:"credit_type"`
	CreditCardBrand          string `db:"credit_card_brand" url:"credit_card_brand"`
	VoucherId                string `db:"voucher_id" url:"voucher_id"`
	StationNumber            string `db:"station_number" url:"station_number"`
	AdditionalDetailsParamX  string `db:"additional_details_param_x" url:"additional_details_param_x"`
	CreditCardCompanyIssuer  string `db:"credit_card_company_issuer" url:"credit_card_company_issuer"`
	DebitCode                string `db:"debit_code" url:"debit_code"`
	FixedPaymentTotal        string `db:"fixed_payment_total" url:"fixed_payment_total"`
	CreditCardNumber         string `db:"credit_card_number" url:"credit_card_number"`
	CreditCardExpDate        string `db:"credit_card_exp_date" url:"credit_card_exp_date"`
	CreditCardCompanyClearer string `db:"credit_card_company_clearer" url:"credit_card_company_clearer"`
	ConfirmationKey          string `db:"-" url:"confirmation_key"`
	DebitTotal               string `db:"debit_total" url:"debit_total"`
	TotalPayments            string `db:"total_payments" url:"total_payments"`
	DebitType                string `db:"debit_type" url:"debit_type"`
	TransactionInitTime      string `db:"transaction_init_time" url:"transaction_init_time"`
	JParam                   string `db:"j_param" url:"j_param"`
	TransactionPelecardId    string `db:"transaction_pelecard_id" url:"transaction_pelecard_id"`
	DebitCurrency            string `db:"debit_currency" url:"debit_currency"`
}
