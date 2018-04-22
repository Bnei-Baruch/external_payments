package types

type ConfirmRequest struct {
	UserKey      string  `json:"userKey" form:"userKey"`
	Price        float64 `json:"price" form:"price" db:"price"`
	Currency     string  `json:"currency" form:"currency" db:"currency"`
	SKU          string  `json:"sku" form:"sku" db:"sku"`
	Reference    string  `json:"reference" form:"reference" db:"reference"`
	Organization string  `json:"organization" form:"organization" db:"organization"`
}

type PaymentRequest struct {
	Id uint64 `json:"-" sql:"id,omitempty"`

	UserKey   string `json:"UserKey" form:"UserKey" db:"user_key" validate:"string,required"`
	CreatedAt string `json:"-" db:"created_at"`
	Status    string `json:"-" db:"status"`

	// Part for Pelecard
	GoodURL   string `json:"GoodURL" form:"GoodURL" db:"good_url" validate:"string,required"`
	ErrorURL  string `json:"ErrorURL" form:"ErrorURL" db:"error_url" validate:"string,required"`
	CancelURL string `json:"CancelURL" form:"CancelURL" db:"cancel_url" validate:"string,required"`

	// Part for Priority
	Name         string  `json:"Name" form:"Name" db:"name" validate:"string,required"`
	Price        float64 `json:"Price" form:"Price" db:"price" validate:"float"`
	Currency     string  `json:"Currency" form:"Currency" db:"currency" validate:"string,required,values=USD|EUR|NIS|ILS"`
	Email        string  `json:"Email" form:"Email" db:"email" validate:"email,required"`
	Phone        string  `json:"Phone" form:"Phone" db:"phone" validate:"string"`
	Street       string  `json:"Street" form:"Street" db:"street" validate:"string,required"`
	City         string  `json:"City" form:"City" db:"city" validate:"string,required"`
	Country      string  `json:"Country" form:"Country" db:"country" validate:"string,required"`
	Participans  string  `json:"Participants" form:"Participants" db:"participants" validate:"string"`
	Details      string  `json:"Details" form:"Details" db:"details" validate:"string"`
	SKU          string  `json:"SKU" form:"SKU" db:"sku" validate:"string,required"`
	VAT          string  `json:"Vat" form:"Vat" db:"vat" validate:"bool,required,values=y|Y|n|N|t|T|f|F"`
	Installments int     `json:"Installments" form:"Installments" db:"installments" validate:"number,min=1,max=12"`
	Language     string  `json:"Language" form:"Language" db:"language" validate:"string,required,values=EN|HE|RU"`
	Reference    string  `json:"Reference" form:"Reference" db:"reference" validate:"string,required"`
	Organization string  `json:"Organization" form:"Organization" db:"organization" validate:"string,required,values=ben2"`
}

// 	CreditType        string  `db:"credit_type"`
// 	CreditCardNumber  string  `db:"credit_card_number"`
// 	CreditCardExpDate string  `db:"credit_card_exp_date"`
// 	FirstPaymentTotal string  `db:"first_payment_total"`
// 	PelecardTransactionId string `db:"pelecard_transaction_id"`
// 	PelecardStatusCode    string `db:"pelecard_status_code"`
// 	ApprovalNo            string `db:"approval_no"`
// 	ConfirmationKey       string `db:"confirmation_key"`
// 	ParamX                string `db:"param_x"`
//
// 	TransactionId            string `db:"transaction_id"`
// 	CardHebrewName           string `db:"card_hebrew_name"`
// 	TransactionUpdateTime    string `db:"transaction_update_time"`
// 	CreditCardAbroadCard     string `db:"credit_card_abroad_card"`
// 	CreditCardBrand          string `db:"credit_card_brand"`
// 	VoucherId                string `db:"voucher_id"`
// 	StationNumber            string `db:"station_number"`
// 	AdditionalDetailsParamX  string `db:"additional_details_param_x"`
// 	CreditCardCompanyIssuer  string `db:"credit_card_company_issuer"`
// 	DebitCode                string `db:"debit_code"`
// 	FixedPaymentTotal        string `db:"fixed_payment_total"`
// 	CreditCardCompanyClearer string `db:"credit_card_company_clearer"`
// 	DebitTotal               string `db:"debit_total"`
// 	TotalPayment             string `db:"total_payments"`
// 	DebitType                string `db:"debit_type"`
// 	TransactionInitTime      string `db:"transaction_init_time"`
// 	JParam                   string `db:"j_param"`
// 	TransactionPelecardId    string `db:"transaction_pelecard_id"`
// 	DebitCurrenct            string `db:"debit_currency"`
// }

type PeleCardResponse struct {
	UserKey               string `db:"user_key"`
	PelecardTransactionId string `db:"pelecard_transaction_id"`
	PelecardStatusCode    string `db:"pelecard_status_code"`
	ConfirmationKey       string `db:"confirmation_key"`
	ParamX                string `db:"param_x"`
}

type PaymentResponse struct {
	UserKey                  string `db:"user_key" url:"user_key"`
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
