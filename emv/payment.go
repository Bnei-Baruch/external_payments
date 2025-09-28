package emv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/go-querystring/query"

	"external_payments/db"
	"external_payments/pelecard"
	"external_payments/types"
	"external_payments/utils"
	"external_payments/validation"
)

func ConfirmPayment(c *gin.Context) {
	var err error
	request := types.ConfirmRequest{}
	if err = c.ShouldBindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBindQuery(&request); err != nil { // Bind by Query String (get)
			utils.ErrorJson("Confirm Bind "+err.Error(), c)
			return
		}
	}
	c.Status(http.StatusOK)
	var message []byte
	if db.Confirm(&request) {
		message = []byte("{\"status\":\"SUCCESS\"}")
		msg := fmt.Sprintf("Confirm Payment FAILURE: %+v", request)
		utils.LogMessage(msg)
	} else {
		message = []byte("{\"status\":\"FAILURE\"}")
		msg := fmt.Sprintf("Confirm Payment SUCCESS: %+v", request)
		utils.LogMessage(msg)
	}
	_, _ = c.Writer.Write(message)
}

func NewPayment(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			utils.ErrorJson("New Bind "+err.Error(), c)
			return
		}
	}
	if errFound, errors := validation.ValidateStruct(request); errFound {
		msg := fmt.Sprintf("New Payment Validation Error: %+v", errors)
		utils.LogMessage(msg)
		utils.ErrorJson("New validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	// Store request into DB
	if err = db.StoreRequest(request); err != nil {
		msg := fmt.Sprintf("New Payment Store Error: %s", err.Error())
		utils.LogMessage(msg)
		utils.ErrorJson("New StoreRequest "+err.Error(), c)
		return
	}

	currency := 1 // ILS
	switch request.Currency {
	case "USD":
		currency = 2
	case "EUR":
		currency = 978
	}

	goodUrl := fmt.Sprintf("https://checkout.kbb1.com/emv/good")
	errorUrl := fmt.Sprintf("https://checkout.kbb1.com/emv/error")
	cancelUrl := fmt.Sprintf("https://checkout.kbb1.com/emv/cancel")

	total := int(float32(request.Price) * 100.00)

	// Request Pelecard
	card := &pelecard.PeleCard{
		Language:    request.Language,
		UserKey:     request.UserKey,
		ParamX:      request.Reference,
		GoodUrl:     goodUrl,
		ErrorUrl:    errorUrl,
		CancelUrl:   cancelUrl,
		Total:       total,
		Currency:    currency,
		MaxPayments: request.Installments,
	}
	if request.Organization == "ben2" {
		card.LogoUrl = "https://checkout.kabbalah.info/logo1.png"
		card.MinPayments = 1
		card.MaxPayments = 1
		if request.Language == "HE" {
			card.TopText = "BB כרטיסי אשראי"
			card.BottomText = "© בני ברוך קבלה לעם"
		} else if request.Language == "RU" {
			card.LogoUrl = "https://checkout.kabbalah.info/kabRu.jpeg"
			card.TopText = "Бней Барух Каббала лаАм"
			card.BottomText = "© Бней Барух Каббала лаАм"
		} else if request.Language == "ES" {
			card.TopText = "Bnei Baruch Kabbalah laAm"
			card.BottomText = "© Bnei Baruch Kabbalah laAm"
			card.Language = "EN"
			card.LogoUrl = "https://cabalacentroestudios.com/wp-content/uploads/2020/04/BB_logo_es.jpg"
			card.CaptionSet = make(map[string]string)
			card.CaptionSet["cs_header_payment"] = "Pago con tarjeta de crédito"
			card.CaptionSet["cs_header_registeration"] = "Registro con tarjeta de crédito"
			card.CaptionSet["cs_holdername"] = "Nombre en la tarjeta"
			card.CaptionSet["cs_cardnumber"] = "Número de tarjeta de crédito"
			card.CaptionSet["cs_expiration"] = "Fecha de expiración"
			card.CaptionSet["cs_id"] = "Pasaporte"
			card.CaptionSet["cs_cvv"] = "CW"
			card.CaptionSet["cs_payments"] = "Número de pagos"
			card.CaptionSet["cs_xparam"] = "Detalles adicionales"
			card.CaptionSet["cs_total"] = "Total"
			card.CaptionSet["cs_supported_cards"] = "Tarjetas aceptadas como pago en este sitio web"
			card.CaptionSet["cs_mustfields"] = "Campos obligatorios"
			card.CaptionSet["cs_submit"] = "Pagar ahora"
			card.CaptionSet["cs_cancel"] = "Cancelar"
		} else {
			card.Language = "EN"
			card.TopText = "BB Credit Cards"
			card.BottomText = "© Bnei Baruch Kabbalah laAm"
		}
	} else if request.Organization == "meshp18" {
		if request.Language == "HE" {
			card.TopText = "משפחה בחיבור כרטיסי אשראי"
			card.BottomText = "© משפחה בחיבור"
		} else if request.Language == "RU" {
			card.TopText = "Бней Барух Каббала лаАм"
			card.BottomText = "© Бней Барух Каббала лаАм"
		} else {
			card.TopText = "BB Credit Cards"
			card.BottomText = "© Bnei Baruch Kabbalah laAm"
		}
		card.MinPayments = 1
		total = total / 100
		if total < 100 {
			card.MaxPayments = 1
		} else {
			card.MaxPayments = total/500 + 2
		}
		if card.MaxPayments > 10 {
			card.MaxPayments = 10
		}
		card.LogoUrl = "https://www.1family.co.il/wp-content/uploads/2019/06/cropped-Screen-Shot-2019-06-16-at-00.12.07-140x82.png"
	} else {
		msg := fmt.Sprintf("New Payment: Unknown Organization")
		utils.LogMessage(msg)
		utils.ErrorJson("Unknown Organization", c)
		return
	}

	if err = card.Init(request.Organization, types.Regular, true); err != nil {
		msg := fmt.Sprintf("New Payment: Pelecard Init %s", err.Error())
		utils.LogMessage(msg)
		utils.ErrorJson("PeleCard Init: "+err.Error(), c)
		return
	}

	if err, url := card.GetRedirectUrl(types.Charge, true); err != nil {
		msg := fmt.Sprintf("New Payment: Error GetRedirectUrl %s", err.Error())
		utils.LogMessage(msg)
		utils.ErrorJson("GetRedirectUrl "+err.Error(), c)
	} else {
		utils.OnRedirect(url, "", "success", c)
	}
}

func GoodPayment(c *gin.Context) {
	var err error

	form := utils.LoadPeleCardForm(c)
	m := fmt.Sprintf("Good Payment: %+v", form)
	utils.LogMessage(m)

	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		m := fmt.Sprintf("Good Payment: %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("UpdateRequestTemp: "+err.Error(), c)
		return
	}

	db.SetStatus(form.UserKey, "in-process")
	// civicrm_bb_ext_requests
	org, err := db.GetOrganization(form.UserKey)
	if err != nil {
		m := fmt.Sprintf("Good Payment: GetOrganization Error %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("GetOrganization: "+err.Error(), c)
		return
	}

	// approve params
	card := &pelecard.PeleCard{}
	if err := card.Init(org, types.Regular, true); err != nil {
		m := fmt.Sprintf("Good Payment: Approve Init Error %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("Approve Init: "+err.Error(), c)
		return
	}

	var msg map[string]interface{}
	if err, msg = card.GetTransaction(form.PelecardTransactionId); err != nil {
		m := fmt.Sprintf("Good Payment: GetTransaction Error %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("GetTransaction: "+err.Error(), c)
		return
	}

	var response = types.PaymentResponse{}
	body, _ := json.Marshal(msg)
	_ = json.Unmarshal(body, &response)
	response.UserKey = form.UserKey
	// update DB
	if err = db.UpdateRequest(response); err != nil {
		m := fmt.Sprintf("Good Payment: Update Request Error %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("UpdateRequest "+err.Error(), c)
		return
	}
	// real validation
	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		m := fmt.Sprintf("Good Payment: Load Request Error %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("LoadRequest "+err.Error(), c)
		return
	}

	if err = card.Init(request.Organization, types.Regular, true); err != nil {
		m := fmt.Sprintf("Good Payment: Validation Init %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("Validation Init "+err.Error(), c)
		return
	}
	card.ConfirmationKey = form.ConfirmationKey
	card.UserKey = request.UserKey
	card.TotalX100 = fmt.Sprintf("%d", int(request.Price*100.00))
	card.Token = form.Token
	card.AuthorizationNumber = form.ApprovalNo
	var valid bool
	if valid, err = card.ValidateByUniqueKey(); err != nil {
		m := fmt.Sprintf("Good Payment: ValidateByUniqueKey 1 error %s", err.Error())
		utils.LogMessage(m)

		db.SetStatus(form.UserKey, "invalid")
		utils.ErrorJson("ValidateByUniqueKey 1 "+err.Error(), c)
		return
	}
	if !valid {
		db.SetStatus(form.UserKey, "invalid")
		m := fmt.Sprintf("Good Payment: Confirmation error 1")
		utils.LogMessage(m)

		utils.ErrorJson("Confirmation error 1 ", c)
		return
	}

	db.SetStatus(form.UserKey, "valid")
	// redirect to GoodURL
	v, _ := query.Values(response)
	utils.OnSuccessPayment(request.GoodURL, v.Encode(), card.Token, card.AuthorizationNumber, c)
}

func Charge(c *gin.Context) {
	var err error

	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			m := fmt.Sprintf("Charge: %s", err.Error())
			utils.LogMessage(m)
			utils.ErrorJson("Charge Bind "+err.Error(), c)
			return
		}
	}

	m := fmt.Sprintf("Charge: %+v", request)
	utils.LogMessage(m)

	// Store request into DB
	if err = db.StoreRequest(request); err != nil {
		m := fmt.Sprintf("Charge: Store request %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("Charge StoreRequest "+err.Error(), c)
		return
	}

	db.SetStatus(request.UserKey, "in-process")

	currency := 1 // ILS
	switch request.Currency {
	case "USD":
		currency = 2
	case "EUR":
		currency = 978
	}
	total := fmt.Sprintf("%d", int(float32(request.Price)*100.00))
	card := &pelecard.PeleCard{
		Token:               request.Token,
		TotalX100:           total,
		Currency:            currency,
		AuthorizationNumber: request.ApprovalNo,
		ParamX:              request.Reference,
	}
	if err = card.Init(request.Organization, types.Regular, true); err != nil {
		m := fmt.Sprintf("Charge: pelecard init %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("Charge PeleCard Init: "+err.Error(), c)
		return
	}

	var msg map[string]interface{}
	var response = types.PaymentResponse{}

	if err, msg = card.ChargeByToken(true); err != nil {
		db.SetStatus(request.UserKey, "invalid")
		m := fmt.Sprintf("Charge: Charge Error %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("Charge error "+err.Error(), c)
		return
	}
	body, _ := json.Marshal(msg)
	_ = json.Unmarshal(body, &response)
	response.UserKey = request.UserKey
	// update DB
	if err = db.UpdateRequest(response); err != nil {
		m := fmt.Sprintf("Charge: UpdateRequest %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("Charge UpdateRequest "+err.Error(), c)
		return
	}

	db.SetStatus(request.UserKey, "valid")
	data, _ := json.Marshal(response)
	var result = map[string]string{
		"status": "success",
		"data":   string(data),
	}
	utils.ResultJson(result, c)
}

func ErrorPayment(c *gin.Context) {
	utils.ErrorPayment(c)
}

func CancelPayment(c *gin.Context) {
	utils.CancelPayment(c)
}
