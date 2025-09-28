package payment

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
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
			OnError("Bind "+err.Error(), c)
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

func GetTransaction(c *gin.Context) {
	var err error
	request := types.GetTransactionRequest{}
	if err = c.ShouldBindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBindQuery(&request); err != nil { // Bind by Query String (get)
			OnError("Bind "+err.Error(), c)
			return
		}
	}
	c.Status(http.StatusOK)
	card := &pelecard.PeleCard{}
	if err = card.Init(request.Organization, types.Regular, true); err != nil {
		OnError("Init"+err.Error(), c)
		return
	}
	var msg map[string]interface{}
	if err, msg = card.GetTransactionData(request.CreatedAt, request.ApprovalNo); err != nil {
		OnError("GetTransactionData "+err.Error(), c)
		return
	}

	body, _ := json.Marshal(msg)
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(body)
}

func NewPayment(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			OnError("Bind "+err.Error(), c)
			return
		}
	}

	if errFound, errors := validation.ValidateStruct(request); errFound {
		OnError("validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	// Store request into DB
	if err = db.StoreRequest(request); err != nil {
		OnError("StoreRequest "+err.Error(), c)
		return
	}

	currency := 1 // ILS
	switch request.Currency {
	case "USD":
		currency = 2
	case "EUR":
		currency = 978
	}

	goodUrl := fmt.Sprintf("https://checkout.kbb1.com/payments/good")
	errorUrl := fmt.Sprintf("https://checkout.kbb1.com/payments/error")
	cancelUrl := fmt.Sprintf("https://checkout.kbb1.com/payments/cancel")

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
		OnError("Unknown Organization", c)
		return
	}

	if err = card.Init(request.Organization, types.Regular, true); err != nil {
		OnError("Init"+err.Error(), c)
		return
	}

	if err, url := card.GetRedirectUrl(types.Charge, true); err != nil {
		OnError("GetRedirectUrl"+err.Error(), c)
	} else {
		OnRedirect(url, "", c)
	}
}

func loadPeleCardForm(c *gin.Context) (form types.PeleCardResponse) {
	form.PelecardTransactionId = c.PostForm("PelecardTransactionId")
	form.PelecardStatusCode = c.PostForm("PelecardStatusCode")
	form.ConfirmationKey = c.PostForm("ConfirmationKey")
	form.ParamX = c.PostForm("ParamX")
	form.UserKey = c.PostForm("UserKey")

	return
}

func GoodPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)

	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		OnError("UpdateRequestTemp: "+err.Error(), c)
		return
	}

	db.SetStatus(form.UserKey, "in-process")
	// civicrm_bb_ext_requests
	org, err := db.GetOrganization(form.UserKey)
	if err != nil {
		OnError("Init"+err.Error(), c)
		return
	}

	// approve params
	card := &pelecard.PeleCard{}
	if err := card.Init(org, types.Regular, true); err != nil {
		OnError("Init"+err.Error(), c)
		return
	}

	var msg map[string]interface{}
	if err, msg = card.GetTransaction(form.PelecardTransactionId); err != nil {
		OnError("GetTransaction "+err.Error(), c)
		return
	}

	var response = types.PaymentResponse{}
	body, _ := json.Marshal(msg)
	_ = json.Unmarshal(body, &response)
	response.UserKey = form.UserKey
	// update DB
	if err = db.UpdateRequest(response); err != nil {
		OnError("UpdateRequest "+err.Error(), c)
		return
	}
	// real validation
	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		OnError("LoadRequest "+err.Error(), c)
		return
	}

	if err := card.Init(request.Organization, types.Regular, true); err != nil {
		OnError("Init "+err.Error(), c)
		return
	}
	card.ConfirmationKey = form.ConfirmationKey
	card.UserKey = request.UserKey
	card.TotalX100 = fmt.Sprintf("%d", int(request.Price*100.00))
	var valid bool
	if valid, err = card.ValidateByUniqueKey(); err != nil {
		db.SetStatus(form.UserKey, "invalid")
		OnError("ValidateByUniqueKey "+err.Error(), c)
		return
	}
	if !valid {
		db.SetStatus(form.UserKey, "invalid")
		OnError("Confirmation error ", c)
		return
	}

	// redirect to GoodURL
	db.SetStatus(form.UserKey, "valid")
	v, _ := query.Values(response)
	OnSuccess(request.GoodURL, v.Encode(), c)
}

func ErrorPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	db.SetStatus(form.UserKey, "error")
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		OnError(err.Error(), c)
		return
	}

	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		OnError(err.Error(), c)
		return
	}
	OnRedirect(request.ErrorURL, pelecard.GetMessage(form.PelecardStatusCode), c)
}

func CancelPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	db.SetStatus(form.UserKey, "cancel")
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		OnError(err.Error(), c)
		return
	}

	var request = types.PaymentRequest{}
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		OnError(err.Error(), c)
		return
	}
	OnRedirect(request.CancelURL, "", c)
}

func OnError(err string, c *gin.Context) {
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte("<html><body><h1 style='color: red;'>Error <code>"))
	_, _ = c.Writer.Write([]byte(err))
	_, _ = c.Writer.Write([]byte("</code></h1><br><pre>"))
	_, _ = c.Writer.Write(debug.Stack())
	_, _ = c.Writer.Write([]byte("</pre></body></html>"))
}

func OnRedirect(url string, msg string, c *gin.Context) {
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
	html := "<script>window.location = '" + target + "';</script>"
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(html))
}

func OnSuccess(url string, msg string, c *gin.Context) {
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

		target = fmt.Sprintf("%s%ssuccess=1&%s", url, q, msg)
	}
	html := "<script>window.location = '" + target + "';</script>"
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(html))
}
