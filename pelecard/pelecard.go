package pelecard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	time "time"

	"github.com/gin-gonic/gin"

	"external_payments/types"
)

type PeleCard struct {
	Url     string `json:"-"`
	Service string `json:"-"`

	User     string `json:"user"`
	Password string `json:"password"`
	Terminal string `json:"terminal"`

	TopText    string `json:",omitempty"`
	BottomText string `json:",omitempty"`
	Language   string `json:",omitempty"`
	LogoUrl    string `json:",omitempty"`

	UserKey   string `json:",omitempty"`
	ParamX    string `json:",omitempty"`
	GoodUrl   string `json:",omitempty"`
	ErrorUrl  string `json:",omitempty"`
	CancelUrl string `json:",omitempty"`

	Total       int `json:",omitempty"`
	Currency    int `json:",omitempty"`
	MinPayments int `json:",omitempty"`
	MaxPayments int `json:",omitempty"`

	ActionType                 string          `json:",omitempty"`
	CreateToken                string          `json:",omitempty"`
	Token                      string          `json:",omitempty"`
	AuthorizationNumber        string          `json:",omitempty"`
	CardHolderName             string          `json:",omitempty"`
	CustomerIdField            string          `json:",omitempty"`
	Cvv2Field                  string          `json:",omitempty"`
	EmailField                 string          `json:",omitempty"`
	TelField                   string          `json:",omitempty"`
	FeedbackDataTransferMethod string          `json:",omitempty"`
	FirstPayment               string          `json:",omitempty"`
	ShopNo                     int             `json:",omitempty"`
	SetFocus                   string          `json:",omitempty"`
	HiddenPelecardLogo         bool            `json:",omitempty"`
	SupportedCards             map[string]bool `json:",omitempty"`

	CaptionSet      map[string]string `json:",omitempty"`
	TransactionId   string            `json:",omitempty"`
	ConfirmationKey string            `json:",omitempty"`
	TotalX100       string            `json:",omitempty"`
}

type service struct {
	TerminalNumber      string
	User                string
	Password            string
	ShopNumber          string `json:",omitempty"`
	Token               string `json:",omitempty"`
	Total               string `json:",omitempty"`
	Currency            int    `json:",omitempty"`
	AuthorizationNumber string `json:",omitempty"`
	ParamX              string `json:",omitempty"`

	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

func (p *PeleCard) Init(organization string, peleCard types.PelecardType, new bool) (err error) {
	p.User = os.Getenv(organization + "_PELECARD_USER")
	p.Password = os.Getenv(organization + "_PELECARD_PASSWORD")
	if peleCard == types.Regular {
		if new {
			p.Terminal = os.Getenv(organization + "_PELECARD_TERMINAL")
		} else {
			p.Terminal = os.Getenv(organization + "_PELECARD_TERMINAL_PREEMV")
		}
	} else {
		p.Terminal = os.Getenv("PELECARD_RECURR_TERMINAL")
	}
	p.Service = os.Getenv("PELECARD_SERVICE_URL")
	p.Url = os.Getenv("PELECARD_URL")
	if p.User == "" || p.Password == "" || p.Terminal == "" ||
		(p.Url == "" && p.Service == "") {
		err = fmt.Errorf("PELECARD parameters are missing %+v", p)
		return
	}

	return
}

func (p *PeleCard) GetTransaction(transactionId string) (err error, msg map[string]interface{}) {

	p.TransactionId = transactionId
	err, msg = p.connect("/GetTransaction")

	return
}

func (p *PeleCard) GetTransactionData(createDate string, approvalNo string) (err error, msg map[string]interface{}) {
	log.Printf("===> GetTransactionData around %s with approval %s\n", createDate, approvalNo)
	layoutIn := "2006-01-02 15:04:05"
	layoutOut := "02/01/2006 15:04"
	date, err := time.Parse(layoutIn, createDate)
	if err != nil {
		log.Printf("===> GetTransactionData time parse error %s\n", err.Error())
		return err, nil
	}
	s := &service{
		TerminalNumber: p.Terminal,
		User:           p.User,
		Password:       p.Password,
	}
	s.StartDate = date.Add(-time.Minute * 5).Format(layoutOut)
	s.EndDate = date.Add(time.Minute * 5).Format(layoutOut)
	log.Printf("===> GetTransactionData between %s and %s\n", s.StartDate, s.EndDate)
	var data []interface{}
	if err, data = p.servicesArr("/GetTransData", s); err != nil {
		return err, nil
	}
	log.Printf("===> GetTransactionData found %d transactions\n", len(data))
	for _, d := range data {
		msg = d.(map[string]interface{})
		log.Printf("===> GetTransactionData transaction with approval %s\n", msg["DebitApproveNumber"])
		if msg["DebitApproveNumber"] == approvalNo {
			log.Printf("===> GetTransactionData FOUND!!!")
			return nil, msg
		}
	}
	log.Printf("===> GetTransactionData NOT FOUND :(")
	return fmt.Errorf("unable to find transaction around %s with approval %s", createDate, approvalNo), nil
}

func (p *PeleCard) GetRedirectUrl(actionType types.ActionType) (err error, url string) {
	p.ActionType = string(actionType)
	if actionType == types.Register {
		p.Cvv2Field = "hide"
	} else {
		p.Cvv2Field = "must"
	}
	p.CreateToken = "True"

	p.CardHolderName = "hide"
	p.CustomerIdField = "hide"
	p.EmailField = "hide"
	p.TelField = "hide"
	p.FeedbackDataTransferMethod = "POST"
	p.FirstPayment = "auto"
	p.ShopNo = 1000
	p.SetFocus = "CC"
	p.HiddenPelecardLogo = true
	p.SupportedCards = map[string]bool{"Amex": true, "Diners": false, "Isra": true, "Master": true, "Visa": true}

	var result map[string]interface{}
	if err, result = p.connect("/init"); err != nil {
		return
	}
	url = result["URL"].(string)
	return
}

func (p *PeleCard) ChargeByToken(skipAuthorizationNumber bool) (err error, result map[string]interface{}) {
	s := &service{
		TerminalNumber: p.Terminal,
		User:           p.User,
		Password:       p.Password,
		ShopNumber:     "1000",
		Token:          p.Token,
		Total:          p.TotalX100,
		Currency:       p.Currency,
		ParamX:         p.ParamX,
	}
	if !skipAuthorizationNumber {
		s.AuthorizationNumber = p.AuthorizationNumber
	}
	err, result = p.services("/DebitRegularType", s)
	return
}

func (p *PeleCard) AuthorizeCreditCard() (err error, result map[string]interface{}) {
	s := &service{
		TerminalNumber: os.Getenv("PELECARD_RECURR_TERMINAL"),
		User:           p.User,
		Password:       p.Password,
		ShopNumber:     "1000",
		Token:          p.Token,
		Total:          "100",
		Currency:       1,
		ParamX:         p.ParamX,
	}
	err, result = p.services("/AuthorizeCreditCard", s)
	return
}

func (p *PeleCard) ValidateByUniqueKey() (valid bool, err error) {
	type validate struct {
		User            string
		Password        string
		Terminal        string
		ConfirmationKey string
		UniqueKey       string
		TotalX100       string
	}

	valid = false
	var v = validate{
		p.User,
		p.Password,
		p.Terminal,
		p.ConfirmationKey,
		p.UserKey,
		p.TotalX100,
	}
	params, _ := json.Marshal(v)
	fmt.Println("https://gateway20.pelecard.biz:443/PaymentGW/ValidateByUniqueKey", "application/json", v)
	resp, err := http.Post("https://gateway20.pelecard.biz:443/PaymentGW/ValidateByUniqueKey", "application/json", bytes.NewBuffer(params))
	if err != nil {
		fmt.Println("Err != nil :(", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Println("StatusCode ", resp.StatusCode)
		return
	}

	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	bodyString := string(bodyBytes)
	if bodyString == "1" {
		valid = true
	}
	return
}

func (p *PeleCard) services(action string, data *service) (err error, result map[string]interface{}) {
	params, _ := json.Marshal(*data)
	url := p.Service + action

	errLogger := gin.DefaultErrorWriter
	var msg string
	msg = fmt.Sprintf("----------> SERVICE: %s\n%s\n", url, params)
	_, _ = errLogger.Write([]byte(msg))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(params))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if status, ok := body["StatusCode"]; ok {
		if status == "000" {
			err = nil
			result = body["ResultData"].(map[string]interface{})
		} else {
			if msg, ok := body["ErrorMessage"]; ok {
				err = fmt.Errorf("0: %s", msg)
			} else {
				err = fmt.Errorf("%s: %s", status, body["ErrorMessage"])
			}
		}
	}

	return
}

func (p *PeleCard) servicesArr(action string, data *service) (err error, result []interface{}) {
	params, _ := json.Marshal(*data)
	url := p.Service + action

	errLogger := gin.DefaultErrorWriter
	var msg string
	msg = fmt.Sprintf("----------> SERVICE: %s\n%s\n", url, params)
	_, _ = errLogger.Write([]byte(msg))

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(params))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if status, ok := body["StatusCode"]; ok {
		if status == "000" {
			err = nil
			result = body["ResultData"].([]interface{})
		} else {
			if msg, ok := body["ErrorMessage"]; ok {
				err = fmt.Errorf("0: %s", msg)
			} else {
				err = fmt.Errorf("%s: %s", status, body["ErrorMessage"])
			}
		}
	}

	return
}

func (p *PeleCard) connect(action string) (err error, result map[string]interface{}) {
	params, _ := json.Marshal(*p)
	url := p.Url + action
	errLogger := gin.DefaultErrorWriter
	m := fmt.Sprintf("----------> CONNECT: %s\n%#v\n", url, p)
	_, _ = errLogger.Write([]byte(m))
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(params))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if urlOk, ok := body["URL"]; ok {
		if urlOk.(string) != "" {
			result = make(map[string]interface{})
			result["URL"] = urlOk.(string)
			return
		}
	}
	if msg, ok := body["Error"]; ok {
		msg := msg.(map[string]interface{})
		if errCode, ok := msg["ErrCode"]; ok {
			if errCode.(float64) > 0 {
				err = fmt.Errorf("%d: %s", int(errCode.(float64)), msg["ErrMsg"])
			}
		} else {
			err = fmt.Errorf("0: %s", msg["ErrMsg"])
		}
	} else {
		if status, ok := body["StatusCode"]; ok {
			if status == "000" {
				err = nil
				result = body["ResultData"].(map[string]interface{})
			} else {
				err = fmt.Errorf("%s: %s", status, body["ErrorMessage"])
			}
		}
	}

	return
}

func (p *PeleCard) connectArr(action string) (err error, result []map[string]interface{}) {
	params, _ := json.Marshal(*p)
	url := p.Url + action
	errLogger := gin.DefaultErrorWriter
	m := fmt.Sprintf("----------> CONNECT: %s\n%#v\n", url, p)
	_, _ = errLogger.Write([]byte(m))
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(params))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	if msg, ok := body["Error"]; ok {
		msg := msg.(map[string]interface{})
		if errCode, ok := msg["ErrCode"]; ok {
			if errCode.(float64) > 0 {
				err = fmt.Errorf("%d: %s", int(errCode.(float64)), msg["ErrMsg"])
			}
		} else {
			err = fmt.Errorf("0: %s", msg["ErrMsg"])
		}
		return
	}
	if status, ok := body["StatusCode"]; ok {
		if status == "000" {
			err = nil
			result = body["ResultData"].([]map[string]interface{})
		} else {
			err = fmt.Errorf("%s: %s", status, body["ErrorMessage"])
		}
	}

	return
}

var messages = map[string]string{
	"000": "Permitted transaction.",
	"001": "The card is blocked, confiscate it.",
	"002": "The card is stolen, confiscate it.",
	"003": "Contact the credit company.",
	"004": "Refusal by credit company.",
	"005": "The card is forged, confiscate it.",
	"006": "Incorrect CVV/ID.",
	"007": "Incorrect CAVV/ECI/UCAF.",
	"008": "An error occurred while building access key for blocked card files.",
	"009": "No communication. Please try again or contact System Administration",
	"010": "The program was stopped by user`s command (ESC) or COM PORT can't be open (Windows)",
	"011": "The acquirer is not authorized for foreign currency transactions",
	"012": "This card is not permitted for foreign currency transactions",
	"013": "The terminal is not permitted for foreign currency charge/discharge into this card",
	"014": "This card is not Supported.",
	"015": "Track 2 (Magnetic) does not match the typed data.",
	"016": "Additional required data was entered/not entered as opposed to terminal Settings (Z field).",
	"017": "Last 4 digits were not entered (W field).",
	"019": "Entry in INT_IN file is shorter than 16 characters.",
	"020": "The input file (INT_IN) does not exist.",
	"021": "Blocked cards file (NEG) does not exist or has not been updated, transmit or request authorization for each transaction.",
	"022": "One of the parameter files/vectors does not exist.",
	"023": "Date file (DATA) does not exist.",
	"024": "Format file (START) does not exist.",
	"025": "The difference in days in the blocked cards input is too large, transmit or request authorization for each transaction.",
	"026": "The difference in generations in the blocked cards input is too large, transmit or request authorization for each transaction.",
	"027": "When the magnetic strip is not completely entered, define the transaction as a telephone number or signature only.",
	"028": "The central terminal number was not entered into the defined main supplier terminal.",
	"029": "The beneficiary number was not entered into the defined main beneficiary terminal.",
	"030": "The supplier/beneficiary number was entered, however the terminal was not updated as the main supplier/beneficiary.",
	"031": "The beneficiary number was entered, however the terminal was updated as the main supplier.",
	"032": "Old transactions, transmit or request authorization for each transaction.",
	"033": "Defective card.",
	"034": "This card is not permitted for this terminal or is not authorized for this type of transaction.",
	"035": "This card is not permitted for this transaction or type of credit.",
	"036": "Expired card.",
	"037": "Installment error, the amount of transactions needs to be equal to: first installment plus fixed installments times number of installments.",
	"038": "Unable to execute a debit transaction that is higher than the credit card`s ceiling.",
	"039": "Incorrect control number.",
	"040": "The beneficiary and supplier numbers were entered, however the terminal is defined as main.",
	"041": "The transaction`s amount exceeds the ceiling when the input file contains J1, J2 or J3 (contact prohibited).",
	"042": "The card is blocked for the supplier where input file contains J1, J2 or J3 (contact prohibited).",
	"043": "Random where input file contains J1 (contact prohibited).",
	"044": "The terminal is prohibited from requesting authorization without transaction (J5).",
	"045": "The terminal is prohibited for supplier-initiated authorization request (J6).",
	"046": "The terminal must request authorization where the input file contains J1, J2 or J3 (contact prohibited).",
	"047": "Secret code must be entered where input file contains J1, J2 or J3 (contact prohibited).",
	"051": "Incorrect vehicle number.",
	"052": "The number of the distance meter was not entered.",
	"053": "The terminal is not defined as gas station (petrol card or incorrect transaction code was used).",
	"057": "An ID number is required (for Israeli cards only) but was not entered.",
	"058": "CVV is required but was not entered.",
	"059": "CVV and ID number are required (for Israeli cards only) but were not entered.",
	"060": "ABS attachment was not found at the beginning of the input data in memory.",
	"061": "The card number was either not found or found twice.",
	"062": "Incorrect transaction type.",
	"063": "Incorrect transaction code.",
	"064": "Incorrect credit type.",
	"065": "Incorrect currency.",
	"066": "The first installment and/or fixed payment are for non-installment type of credit.",
	"067": "Number of installments exist for the type of credit that does not require this.",
	"068": "Linkage to dollar or index is possible only for installment credit.",
	"069": "The magnetic strip is too short.",
	"070": "The PIN code device is not defined.",
	"071": "Must enter the PIN code number.",
	"072": "Smart card reader not available - use the magnetic reader.",
	"073": "Must use the Smart card reader.",
	"074": "Denied - locked card.",
	"075": "Denied - Smart card reader action didn't end in the correct time.",
	"076": "Denied - Data from smart card reader not defined in system.",
	"077": "Incorrect PIN code.",
	"079": "Currency does not exist in vector 59.",
	"080": "The club code entered does not match the credit type.",
	"090": "Cannot cancel charge transaction.Make charging deal.",
	"091": "Cannot cancel charge transaction.Make discharge transaction ",
	"092": "Cannot cancel charge transaction.Please create a credit transaction.",
	"099": "Unable to read/write/open the TRAN file.",
	"101": "No authorization from credit company for clearance. ",
	"106": "The terminal is not permitted to send queries for immediate debit cards.",
	"107": "The transaction amount is too large, divide it into a number of transactions.",
	"108": "The terminal is not authorized to execute forced transactions.",
	"109": "The terminal is not authorized for cards with service code 587.",
	"110": "The terminal is not authorized for immediate debit cards.",
	"111": "The terminal is not authorized for installment transactions.",
	"112": "The terminal is authorized for installment transactions only",
	"113": "The terminal is not authorized for telephone transactions.",
	"114": "The terminal is not authorized for signature-only transactions.",
	"115": "The terminal is not authorized for foreign currency transactions, or transaction is not authorized.",
	"116": "The terminal is not authorized for club transactions.",
	"117": "The terminal is not authorized for star /point/mile transactions.",
	"118": "The terminal is not authorized for Isracredit credit.",
	"119": "The terminal is not authorized for Amex credit.",
	"120": "The terminal is not authorized for dollar linkage.",
	"121": "The terminal is not authorized for index linkage.",
	"122": "The terminal is not authorized for index linkage with foreign cards.",
	"123": "The terminal is not authorized for star",
	"124": "The terminal is not authorized for Isra 36 credit.",
	"125": "The terminal is not authorized for Amex 36 credit.",
	"126": "The terminal is not authorized for this club code.",
	"127": "The terminal is not authorized for immediate debit transactions (except for immediate debit cards ).",
	"128": "The terminal is not authorized to accept Visa card staring with 3.",
	"129": "The terminal is not authorized to execute credit transactions above the ceiling.",
	"130": "The card is not permitted to execute club transactions.",
	"131": "The card is not permitted to execute star/point/mile transactions.",
	"132": "The card is not permitted to execute dollar transactions (regular or telephone).",
	"133": "The card is not valid according to Isracard `s list of valid cards.",
	"134": "Defective card according to system definitions (Isracard VECTOR1), error in the number of figures on the card.",
	"135": "The card is not permitted to execute dollar transactions according to system definitions (Isracard VECTOR1).",
	"136": "The card belongs to a group that is not permitted to execute transactions according to system definitions (Visa VECTOR 20).",
	"137": "The card` s prefix (7 figures) is invalid according to system definitions (Diners VECTOR21).",
	"138": "The card is not permitted to carry out installment transactions according to Isracard `s list of valid cards.",
	"139": "The number of installments is too large according to Isracard` s list of valid cards.",
	"140": "Visa and Diners cards are not permitted for club installment transactions.",
	"141": "Series of cards are not valid according to system definition (Isracard VECTOR5).",
	"142": "Invalid service code according to system definitions (Isracard VECTOR6).",
	"143": "The card `s prefix (2 figures) is invalid according to system definitions (Isracard VECTOR7).",
	"144": "Invalid service code according to system definitions (Visa VECTOR12).",
	"145": "Invalid service code according to system definitions (Visa VECTOR13).",
	"146": "Immediate debit card is prohibited for executing credit transaction.",
	"147": "The card is not permitted to execute installment transactions according to Alpha vector no. 31.",
	"148": "The card is not permitted for telephone and signature-only transactions according to Alpha vector no. 31.",
	"149": "The card is not permitted for telephone transactions according to Alpha vector no. 31.",
	"150": "Credit is not approved for immediate debit cards.",
	"151": "Credit is not approved for foreign cards.",
	"152": "Incorrect club code.",
	"153": "The card is not permitted to execute flexible credit transactions (Adif/30+) according to system definitions (Diners VECTOR21).",
	"154": "The card is not permitted to execute immediate debit transactions according to system definitions (Diners VECTOR21).",
	"155": "The payment amount is too low for credit transactions.",
	"156": "Incorrect number of installments for credit transaction.",
	"157": "Zero ceiling for this type of card for regular credit or Credit transaction.",
	"158": "ero ceiling for this type of card for immediate debit credit transaction.",
	"159": "Zero ceiling for this type of card for immediate debit in dollars.",
	"160": "Zero ceiling for this type of card for telephone transaction.",
	"161": "Zero ceiling for this type of card for credit transaction.",
	"162": "Zero ceiling for this type of card for installment transaction.",
	"163": "American Express card issued abroad not permitted for instalments transaction.",
	"164": "JCB cards are only permitted to carry out regular credit transactions.",
	"165": "The amount in stars/points/miles is higher than the transaction amount.",
	"166": "The club card is not within terminal range.",
	"167": "Star/point/mile transactions cannot be executed.",
	"168": "Dollar transactions cannot be executed for this type of card.",
	"169": "Credit transactions cannot be executed with other than regular credit.",
	"170": "Amount of discount on stars/points/miles is higher than the permitted.",
	"171": "Forced transactions cannot be executed with credit/immediate debit card.",
	"172": "The previous transaction cannot be cancelled (credit transaction or card number are not identical).",
	"173": "Double transaction.",
	"174": "The terminal is not permitted for index linkage of this type of credit.",
	"175": "The terminal is not permitted for dollar linkage of this type of credit.",
	"176": "The card is invalid according to system definitions (Isracard VECTOR1).",
	"177": "Unable to execute the self-service transaction if the gas station does not have self service.",
	"178": "Credit transactions are forbidden with stars/points/miles.",
	"179": "Dollar credit transactions are forbidden on tourist cards.",
	"180": "Phone transactions are not permitted on Club cards.",
	"200": "Application error.",
	"201": "Error receiving encrypted data",
	"205": "Transaction amount missing or zero.",
	"301": "Timeout on clearing page.",
	"306": "No communication to Pelecard.",
	"308": "Doubled transaction.",
	"404": "Terminal number does not exist. ",
	"500": "Terminal executes broadcast and/or updating data. Please try again later. ",
	"501": "User name and/or password not correct. Please call support team. ",
	"502": "User password has expired. Please contact support team. ",
	"503": "Locked user. Please contact support team. ",
	"505": "Blocked terminal. Please contact account team. ",
	"506": "Token number abnormal. ",
	"507": "User is not authorized in this terminal. ",
	"508": "Validity structure invalid. Use MMYY structure only. ",
	"509": "SSL verifying access is blocked. Please contact support team. ",
	"510": "Data not exist. ",
	"555": "Cancel url status code.",
	"597": "General error. Please contact support team. ",
	"598": "Necessary values are missing/wrong. ",
	"599": "General error. Repeat action. ",
	"999": "Necessary values missing to complete installments transaction.",
}

func GetMessage(value string) string {
	return messages[value]
}
