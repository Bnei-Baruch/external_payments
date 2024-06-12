package emv

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"external_payments/db"
	"external_payments/pelecard"
	"external_payments/types"
	"external_payments/validation"
)

func NewToken(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			ErrorJson("New Bind "+err.Error(), c)
			return
		}
	}
	msg := fmt.Sprintf("NewToken: %+v", request)
	logMessage(msg)
	if errFound, errors := validation.ValidateStruct(request); errFound {
		msg := fmt.Sprintf("NewToken Validation Error: %+v", errors)
		logMessage(msg)
		ErrorJson("New validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	// Store request into DB
	if err = db.StoreRequest(request); err != nil {
		msg := fmt.Sprintf("NewToken Store Error: %s", err.Error())
		logMessage(msg)
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

	goodUrl := fmt.Sprintf("https://checkout.kbb1.com/emv/good_token")
	errorUrl := fmt.Sprintf("https://checkout.kbb1.com/emv/error")
	cancelUrl := fmt.Sprintf("https://checkout.kbb1.com/emv/cancel")

	// Request Pelecard
	card := &pelecard.PeleCard{
		Language:    request.Language,
		UserKey:     request.UserKey,
		ParamX:      request.Reference,
		GoodUrl:     goodUrl,
		ErrorUrl:    errorUrl,
		CancelUrl:   cancelUrl,
		Total:       0,
		Currency:    currency,
		MaxPayments: 1,
		MinPayments: 1,
	}
	if request.Organization == "ben2" {
		card.LogoUrl = "https://checkout.kabbalah.info/logo1.png"
		if request.Language == "HE" {
			card.TopText = "BB כרטיסי אשראי"
			card.BottomText = "© בני ברוך קבלה לעם"
			card.CaptionSet = make(map[string]string)
			card.CaptionSet["cs_submit"] = "שמור"
		} else if request.Language == "RU" {
			card.LogoUrl = "https://checkout.kabbalah.info/kabRu.jpeg"
			card.TopText = "Бней Барух Каббала лаАм"
			card.BottomText = "© Бней Барух Каббала лаАм"
			card.CaptionSet = make(map[string]string)
			card.CaptionSet["cs_submit"] = "Сохранить"
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
			card.CaptionSet["cs_submit"] = "Ahorrar"
			card.CaptionSet["cs_cancel"] = "Cancelar"
		} else {
			card.Language = "EN"
			card.TopText = "BB Credit Cards"
			card.BottomText = "© Bnei Baruch Kabbalah laAm"
			card.CaptionSet = make(map[string]string)
			card.CaptionSet["cs_submit"] = "Save"
		}
	} else if request.Organization == "meshp18" {
		if request.Language == "HE" {
			card.TopText = "משפחה בחיבור כרטיסי אשראי"
			card.BottomText = "© משפחה בחיבור"
			card.CaptionSet = make(map[string]string)
			card.CaptionSet["cs_submit"] = "שמור"
		} else if request.Language == "RU" {
			card.TopText = "Бней Барух Каббала лаАм"
			card.BottomText = "© Бней Барух Каббала лаАм"
			card.CaptionSet = make(map[string]string)
			card.CaptionSet["cs_submit"] = "Сохранить"
		} else {
			card.TopText = "BB Credit Cards"
			card.BottomText = "© Bnei Baruch Kabbalah laAm"
			card.CaptionSet = make(map[string]string)
			card.CaptionSet["cs_submit"] = "Save"
		}
		card.LogoUrl = "https://www.1family.co.il/wp-content/uploads/2019/06/cropped-Screen-Shot-2019-06-16-at-00.12.07-140x82.png"
	} else {
		msg := fmt.Sprintf("NewToken: Unknown Organization")
		logMessage(msg)
		ErrorJson("Unknown Organization", c)
		return
	}

	if err = card.Init(request.Organization, types.Regular, true); err != nil {
		msg := fmt.Sprintf("NewToken: Pelecard Init %s", err.Error())
		logMessage(msg)

		ErrorJson("PeleCard Init: "+err.Error(), c)
		return
	}

	if err, url := card.GetRedirectUrl(types.Register); err != nil {
		msg := fmt.Sprintf("NewToken: Error GetRedirectUrl %s", err.Error())
		logMessage(msg)

		ErrorJson("GetRedirectUrl "+err.Error(), c)
	} else {
		OnRedirect(url, "", "success", c)
	}
}

func GoodToken(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	m := fmt.Sprintf("Good Token: %+v", form)
	logMessage(m)

	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		m := fmt.Sprintf("Good Token: %s", err.Error())
		logMessage(m)
		ErrorJson("UpdateRequestTemp: "+err.Error(), c)
		return
	}

	db.SetStatus(form.UserKey, "valid")

	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		m := fmt.Sprintf("Good Payment: Load Request Error %s", err.Error())
		logMessage(m)

		ErrorJson("LoadRequest "+err.Error(), c)
		return
	}
	// redirect to GoodURL
	onSuccessToken(request.GoodURL, form.Token, form.ParamX, c)
}

func onSuccessToken(url string, token string, paramX string, c *gin.Context) {
	target := fmt.Sprintf("%s?token=%s&paramX=%s", url, token, paramX)
	html := "<script>window.location = '" + target + "';</script>"
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write([]byte(html))
}
