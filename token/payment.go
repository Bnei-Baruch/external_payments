package token

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
	"external_payments/validation"
)

func ConfirmPayment(c *gin.Context) {
	var err error
	request := types.ConfirmRequest{}
	if err = c.ShouldBindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBindQuery(&request); err != nil { // Bind by Query String (get)
			ErrorJson("Confirm Bind "+err.Error(), c)
			return
		}
	}

	c.Status(http.StatusOK)
	var message []byte
	if db.Confirm(&request) {
		message = []byte("status=SUCCESS")
	} else {
		message = []byte("status=FAILURE")
	}
	_, _ = c.Writer.Write(message)
}

func NewPayment(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			ErrorJson("New Bind "+err.Error(), c)
			return
		}
	}

	if errFound, errors := validation.ValidateStruct(request); errFound {
		ErrorJson("New validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	// Store request into DB
	if err = db.StoreRequest(request); err != nil {
		ErrorJson("New StoreRequest "+err.Error(), c)
		return
	}

	currency := 1 // ILS
	switch request.Currency {
	case "USD":
		currency = 2
	case "EUR":
		currency = 978
	}

	goodUrl := fmt.Sprintf("https://checkout.kbb1.com/token/good")
	errorUrl := fmt.Sprintf("https://checkout.kbb1.com/token/error")
	cancelUrl := fmt.Sprintf("https://checkout.kbb1.com/token/cancel")

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
			card.LogoUrl = "http://cabalacentroestudios.com/wp-content/uploads/2020/04/BB_logo_es.jpg"
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
		ErrorJson("Unknown Organization", c)
		return
	}

	if err = card.Init(request.Organization, types.Recurrent); err != nil {
		ErrorJson("PeleCard Init: "+err.Error(), c)
		return
	}

	if err, url := card.GetRedirectUrl(true); err != nil {
		ErrorJson("GetRedirectUrl "+err.Error(), c)
	} else {
		OnRedirect(url, "", "success", c)
	}
}

func loadPeleCardForm(c *gin.Context) (form types.PeleCardResponse) {
	form.PelecardTransactionId = c.PostForm("PelecardTransactionId")
	form.PelecardStatusCode = c.PostForm("PelecardStatusCode")
	form.ConfirmationKey = c.PostForm("ConfirmationKey")
	form.Token = c.PostForm("Token")
	form.ApprovalNo = c.PostForm("ApprovalNo")
	form.ParamX = c.PostForm("ParamX")
	form.UserKey = c.PostForm("UserKey")

	return
}

func GoodPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)

	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		ErrorJson("UpdateRequestTemp: "+err.Error(), c)
		return
	}

	db.SetStatus(form.UserKey, "in-process")
	// bb_ext_requests
	org, err := db.GetOrganization(form.UserKey)
	if err != nil {
		ErrorJson("GetOrganization: "+err.Error(), c)
		return
	}

	// approve params
	card := &pelecard.PeleCard{}
	if err := card.Init(org, types.Recurrent); err != nil {
		ErrorJson("Approve Init: "+err.Error(), c)
		return
	}

	var msg map[string]interface{}
	if err, msg = card.GetTransaction(form.PelecardTransactionId); err != nil {
		ErrorJson("GetTransaction: "+err.Error(), c)
		return
	}

	var response = types.PaymentResponse{}
	body, _ := json.Marshal(msg)
	_ = json.Unmarshal(body, &response)
	response.UserKey = form.UserKey
	// update DB
	if err = db.UpdateRequest(response); err != nil {
		ErrorJson("UpdateRequest "+err.Error(), c)
		return
	}
	// real validation
	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		ErrorJson("LoadRequest "+err.Error(), c)
		return
	}

	if err = card.Init(request.Organization, types.Regular); err != nil {
		ErrorJson("Validation Init "+err.Error(), c)
		return
	}
	card.ConfirmationKey = form.ConfirmationKey
	card.UserKey = request.UserKey
	card.TotalX100 = fmt.Sprintf("%d", int(request.Price*100.00))
	card.Token = form.Token
	card.AuthorizationNumber = form.ApprovalNo
	var valid bool
	if valid, err = card.ValidateByUniqueKey(); err != nil {
		db.SetStatus(form.UserKey, "invalid")
		ErrorJson("ValidateByUniqueKey 1 "+err.Error(), c)
		return
	}
	if !valid {
		db.SetStatus(form.UserKey, "invalid")
		ErrorJson("Confirmation error 1 ", c)
		return
	}

	// Charge donor for the first time
	currency := 1 // ILS
	switch request.Currency {
	case "USD":
		currency = 2
	case "EUR":
		currency = 978
	}
	card = &pelecard.PeleCard{
		Currency:            currency,
		UserKey:             request.UserKey,
		Token:               form.Token,
		AuthorizationNumber: form.ApprovalNo,
		ParamX:              form.ParamX,
		TotalX100:           fmt.Sprintf("%d", int(request.Price*100.00)),
	}
	if err := card.Init(org, types.Regular); err != nil {
		ErrorJson("Approve Init: "+err.Error(), c)
		return
	}
	if err, msg = card.ChargeByToken(); err != nil {
		db.SetStatus(form.UserKey, "invalid")
		ErrorJson("First Charge error ", c)
		return
	}

	db.SetStatus(form.UserKey, "valid")
	// redirect to GoodURL
	v, _ := query.Values(response)
	OnSuccess(request.GoodURL, v.Encode(), card.Token, card.AuthorizationNumber, c)
}

func Charge(c *gin.Context) {
	var err error

	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			ErrorJson("Charge Bind "+err.Error(), c)
			return
		}
	}

	// Store request into DB
	if err = db.StoreRequest(request); err != nil {
		ErrorJson("Charge StoreRequest "+err.Error(), c)
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
	if err = card.Init(request.Organization, types.Regular); err != nil {
		ErrorJson("Charge PeleCard Init: "+err.Error(), c)
		return
	}

	var msg map[string]interface{}
	var response = types.PaymentResponse{}

	if err, msg = card.ChargeByToken(); err != nil {
		db.SetStatus(request.UserKey, "invalid")
		ErrorJson("Charge error ", c)
		return
	}
	body, _ := json.Marshal(msg)
	_ = json.Unmarshal(body, &response)
	response.UserKey = request.UserKey
	// update DB
	if err = db.UpdateRequest(response); err != nil {
		ErrorJson("Charge UpdateRequest "+err.Error(), c)
		return
	}

	db.SetStatus(request.UserKey, "valid")
	var result = map[string]string{
		"status": "success",
	}
	ResultJson(result, c)
}

func ErrorJson(message string, c *gin.Context) {
	msg := map[string]string{
		"status": "error",
		"error":  message,
	}
	ResultJson(msg, c)
}

func ErrorPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	db.SetStatus(form.UserKey, "error")
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		ErrorJson(err.Error(), c)
		return
	}

	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		ErrorJson(err.Error(), c)
		return
	}
	OnRedirect(request.ErrorURL, pelecard.GetMessage(form.PelecardStatusCode), "error", c)
}

func CancelPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	db.SetStatus(form.UserKey, "cancel")
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		ErrorJson(err.Error(), c)
		return
	}

	var request = types.PaymentRequest{}
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		ErrorJson(err.Error(), c)
		return
	}
	OnRedirect(request.CancelURL, "", "cancel", c)
}

func OnRedirect(url string, msg string, status string, c *gin.Context) {
	var target string
	if msg == "" {
		target = url
	} else {
		var q string
		if strings.ContainsRune(url, '?') {
			q = "&"
		} else {
			q = "?"
		}

		target = fmt.Sprintf("%s%serror=%s", url, q, msg)
	}
	result := map[string]string{
		"status": status,
		"url":    target,
	}
	ResultJson(result, c)
}

func OnSuccess(url string, msg string, token string, authNo string, c *gin.Context) {
	var target string
	if msg == "" {
		target = fmt.Sprintf("%s?token=%s&authNo=%s", url, token, authNo)
	} else {
		var q string
		if strings.ContainsRune(url, '?') {
			q = "&"
		} else {
			q = "?"
		}

		target = fmt.Sprintf("%s%ssuccess=1&token=%s&authNo=%s&%s", url, q, token, authNo, msg)
	}
	result := map[string]string{
		"status": "success",
		"url":    target,
	}
	ResultJson(result, c)
}

func ResultJson(msg map[string]string, c *gin.Context) {
	js, _ := json.Marshal(msg)
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(js)
}
