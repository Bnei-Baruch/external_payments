package emv

import (
	"encoding/json/v2"
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"external_payments/db"
	"external_payments/pelecard"
	"external_payments/types"
	"external_payments/utils"
	"external_payments/validation"
)

func NewToken(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			utils.ErrorJson("New Bind "+err.Error(), c)
			return
		}
	}
	msg := fmt.Sprintf("NewToken: %+v", request)
	utils.LogMessage(msg)
	if errFound, errors := validation.ValidateStruct(request); errFound {
		msg := fmt.Sprintf("NewToken Validation Error: %+v", errors)
		utils.LogMessage(msg)
		utils.ErrorJson("New validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	// Store request into DB
	if err = db.StoreRequest(request); err != nil {
		msg := fmt.Sprintf("NewToken Store Error: %s", err.Error())
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

	baseUrl := os.Getenv("EXT_BASE_URL")
	if baseUrl == "" {
		baseUrl = "https://checkout.kbb1.com"
	}

	// Request Pelecard
	card := &pelecard.PeleCard{
		Language:    request.Language,
		UserKey:     request.UserKey,
		ParamX:      request.Reference,
		GoodUrl:     baseUrl + "/emv/good_token",
		ErrorUrl:    baseUrl + "/emv/error",
		CancelUrl:   baseUrl + "/emv/cancel",
		Total:       0,
		Currency:    currency,
		MaxPayments: 1,
		MinPayments: 1,

		CaptionSet: make(map[string]string),
	}
	if request.Organization == "ben2" {
		card.LogoUrl = "https://checkout.kabbalah.info/logo1.png"
		if request.Language == "HE" {
			card.TopText = "BB כרטיסי אשראי"
			card.BottomText = "© בני ברוך קבלה לעם"
			card.CaptionSet["cs_submit"] = "שמור"
		} else if request.Language == "RU" {
			card.LogoUrl = "https://checkout.kabbalah.info/kabRu.jpeg"
			card.TopText = "Бней Барух Каббала лаАм"
			card.BottomText = "© Бней Барух Каббала лаАм"
			card.CaptionSet["cs_submit"] = "Сохранить"
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
			card.CaptionSet["cs_submit"] = "Ahorrar"
			card.CaptionSet["cs_cancel"] = "Cancelar"
		} else {
			card.Language = "EN"
			card.TopText = "BB Credit Cards"
			card.BottomText = "© Bnei Baruch Kabbalah laAm"
			card.CaptionSet["cs_submit"] = "Save"
		}
	} else if request.Organization == "meshp18" {
		if request.Language == "HE" {
			card.TopText = "משפחה בחיבור כרטיסי אשראי"
			card.BottomText = "© משפחה בחיבור"
			card.CaptionSet["cs_submit"] = "שמור"
		} else if request.Language == "RU" {
			card.TopText = "Бней Барух Каббала лаАм"
			card.BottomText = "© Бней Барух Каббала лаАм"
			card.CaptionSet["cs_submit"] = "Сохранить"
		} else {
			card.TopText = "BB Credit Cards"
			card.BottomText = "© Bnei Baruch Kabbalah laAm"
			card.CaptionSet["cs_submit"] = "Save"
		}
		card.LogoUrl = "https://www.1family.co.il/wp-content/uploads/2019/06/cropped-Screen-Shot-2019-06-16-at-00.12.07-140x82.png"
	} else {
		msg := fmt.Sprintf("NewToken: Unknown Organization")
		utils.LogMessage(msg)
		utils.ErrorJson("Unknown Organization", c)
		return
	}

	if err = card.Init(request.Organization, types.Regular, true); err != nil {
		msg := fmt.Sprintf("NewToken: Pelecard Init %s", err.Error())
		utils.LogMessage(msg)

		utils.ErrorJson("PeleCard Init: "+err.Error(), c)
		return
	}

	if err, url := card.GetRedirectUrl(types.Register, true); err != nil {
		msg := fmt.Sprintf("NewToken: Error GetRedirectUrl %s", err.Error())
		utils.LogMessage(msg)

		utils.ErrorJson("GetRedirectUrl "+err.Error(), c)
	} else {
		utils.OnRedirect(url, "", "success", c)
	}
}

func GoodToken(c *gin.Context) {
	var err error

	form := utils.LoadPeleCardForm(c)
	m := fmt.Sprintf("Good Token: %+v", form)
	utils.LogMessage(m)

	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		m := fmt.Sprintf("Good Token: %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("UpdateRequestTemp: "+err.Error(), c)
		return
	}

	db.SetStatus(form.UserKey, "valid")

	org, err := db.GetOrganization(form.UserKey)
	if err != nil {
		m := fmt.Sprintf("Good Token: GetOrganization Error %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("GetOrganization: "+err.Error(), c)
		return
	}
	card := &pelecard.PeleCard{}
	if err := card.Init(org, types.Regular, true); err != nil {
		m := fmt.Sprintf("Good Token: Approve Init Error %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("Approve Init: "+err.Error(), c)
		return
	}
	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		m := fmt.Sprintf("Good Token: Load Request Error %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("LoadRequest "+err.Error(), c)
		return
	}

	card.ConfirmationKey = form.ConfirmationKey
	card.UserKey = request.UserKey
	card.TotalX100 = fmt.Sprintf("%d", int(request.Price*100.00))
	var valid bool
	if valid, err = card.ValidateByUniqueKey(); err != nil {
		m := fmt.Sprintf("Good Token: ValidateByUniqueKey error %s", err.Error())
		utils.LogMessage(m)
		db.SetStatus(form.UserKey, "invalid")
		utils.ErrorJson("ValidateByUniqueKey "+err.Error(), c)
		return
	}
	if !valid {
		db.SetStatus(form.UserKey, "invalid")
		utils.LogMessage("Good Token: Confirmation error")
		utils.ErrorJson("Confirmation error", c)
		return
	}

	var msg map[string]any
	if err, msg = card.GetTransaction(form.PelecardTransactionId); err != nil {
		m := fmt.Sprintf("Good Token: GetTransaction Error %s", err.Error())
		utils.LogMessage(m)
		utils.ErrorJson("GetTransaction: "+err.Error(), c)
		return
	}
	var response = types.PaymentResponse{}
	body, _ := json.Marshal(msg)
	_ = json.Unmarshal(body, &response)

	// redirect to GoodURL
	utils.OnSuccessToken(request.GoodURL, form, response, c)
}
