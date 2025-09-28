package renew_card

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"external_payments/db"
	"external_payments/payment"
	"external_payments/pelecard"
	"external_payments/types"
	"external_payments/utils"
	"external_payments/validation"
)

func RenewCard(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			payment.OnError("Bind "+err.Error(), c)
			return
		}
	}

	if errFound, errors := validation.ValidateStruct(request); errFound {
		msg := fmt.Sprintf("New J2 Validation Error: %+v", errors)
		utils.LogMessage(msg)
		utils.ErrorJson("New validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	// Store request into DB
	if err = db.StoreRequest(request); err != nil {
		msg := fmt.Sprintf("New J2 Store Error: %s", err.Error())
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

	total := int(float32(request.Price) * 100.00)

	// Request Pelecard
	card := &pelecard.PeleCard{
		Language:    request.Language,
		UserKey:     request.UserKey,
		ParamX:      request.Reference,
		GoodUrl:     "https://checkout.kbb1.com/renew/good",
		ErrorUrl:    "https://checkout.kbb1.com/renew/error",
		CancelUrl:   "https://checkout.kbb1.com/renew/cancel",
		Total:       total,
		Currency:    currency,
		MaxPayments: request.Installments,
		CaptionSet:  make(map[string]string),
	}
	if request.Organization == "ben2" {
		card.LogoUrl = "https://checkout.kabbalah.info/logo1.png"
		card.MinPayments = 1
		card.MaxPayments = 1
		if request.Language == "HE" {
			card.TopText = "BB כרטיסי אשראי"
			card.BottomText = "© בני ברוך קבלה לעם"
			card.CaptionSet["cs_submit"] = "Renew"
			card.CaptionSet["cs_cancel"] = "Cancel"
		} else if request.Language == "RU" {
			card.LogoUrl = "https://checkout.kabbalah.info/kabRu.jpeg"
			card.TopText = "Бней Барух Каббала лаАм"
			card.BottomText = "© Бней Барух Каббала лаАм"
		} else if request.Language == "ES" {
			card.TopText = "Bnei Baruch Kabbalah laAm"
			card.BottomText = "© Bnei Baruch Kabbalah laAm"
			card.Language = "EN"
			card.LogoUrl = "https://cabalacentroestudios.com/wp-content/uploads/2020/04/BB_logo_es.jpg"
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

	if err, url := card.GetRedirectUrl(types.Register, false); err != nil {
		msg := fmt.Sprintf("New Payment: Error GetRedirectUrl %s", err.Error())
		utils.LogMessage(msg)
		utils.ErrorJson("GetRedirectUrl "+err.Error(), c)
	} else {
		utils.OnRedirect(url, "", "success", c)
	}
}

func GoodJ2(c *gin.Context) {
	var err error

	form := utils.LoadPeleCardForm(c)
	m := fmt.Sprintf("Good J2: %+v", form)
	utils.LogMessage(m)

	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		m := fmt.Sprintf("Good J2: %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("UpdateRequestTemp J2: "+err.Error(), c)
		return
	}

	db.SetStatus(form.UserKey, "valid")

	org, err := db.GetOrganization(form.UserKey)
	if err != nil {
		m := fmt.Sprintf("Good J2: GetOrganization Error %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("GetOrganization: "+err.Error(), c)
		return
	}

	// approve params
	card := &pelecard.PeleCard{}
	if err := card.Init(org, types.Regular, true); err != nil {
		m := fmt.Sprintf("Good J2: Approve Init Error %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("Approve Init: "+err.Error(), c)
		return
	}

	var msg map[string]interface{}
	if err, msg = card.GetTransaction(form.PelecardTransactionId); err != nil {
		m := fmt.Sprintf("Good J2: GetTransaction Error %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("GetTransaction: "+err.Error(), c)
		return
	}

	var response = types.PaymentResponse{}
	body, _ := json.Marshal(msg)
	_ = json.Unmarshal(body, &response)

	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		m := fmt.Sprintf("Good J2: Load Request Error %s", err.Error())
		utils.LogMessage(m)

		utils.ErrorJson("LoadRequest "+err.Error(), c)
		return
	}

	// redirect to GoodURL
	utils.OnSuccessToken(request.GoodURL, form, response, c)
}
