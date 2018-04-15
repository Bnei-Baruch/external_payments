package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/go-querystring/query"

	"github.com/gshilin/external_payments/db"
	"github.com/gshilin/external_payments/pelecard"
	"github.com/gshilin/external_payments/types"
	"runtime/debug"
)

func NewPayment(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			onError("Bind"+err.Error(), c)
			return
		}
	}

	fmt.Printf("Request: %#v\n", request)
	if errFound, errors := validateStruct(request); errFound {
		onError("validateStruct "+strings.Join(errors, "\n"), c)
		return
	}

	// Store request into DB
	if _, err = db.StoreRequest(request); err != nil {
		onError("StoreRequest"+err.Error(), c)
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

	// Request Pelecard
	card := &pelecard.PeleCard{
		Language:    request.Language,
		UserKey:     request.UserKey,
		GoodUrl:     goodUrl,
		ErrorUrl:    errorUrl,
		CancelUrl:   cancelUrl,
		Total:       int(request.Price * 100.00),
		Currency:    currency,
		MaxPayments: request.Installments,
	}
	if request.Language == "HE" {
		card.TopText = "BB כרטיסי אשראי"
		card.BottomText = "© בני ברוך קבלה לעם"
	} else if request.Language == "RU" {
		card.TopText = "Бней Барух Каббала лаАм"
		card.BottomText = "© Бней Барух Каббала лаАм"
	} else {
		card.TopText = "BB Credit Cards"
		card.BottomText = "© Bnei Baruch Kabbalah laAm"
	}

	if err = card.Init(); err != nil {
		onError("Init"+err.Error(), c)
		return
	}

	if err, url := card.GetRedirectUrl(); err != nil {
		onError("GetRedirectUrl"+err.Error(), c)
	} else {
		onRedirect(url, "", c)
	}
}

func loadPeleCardForm(c *gin.Context) (form types.PeleCardResponse) {
	form.PelecardTransactionId = c.PostForm("PelecardTransactionId")
	form.PelecardStatusCode = c.PostForm("PelecardStatusCode")
	form.ApprovalNo = c.PostForm("ApprovalNo")
	form.ConfirmationKey = c.PostForm("ConfirmationKey")
	form.ParamX = c.PostForm("ParamX")
	form.UserKey = c.PostForm("UserKey")

	return
}

func GoodPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)

	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		onError("UpdateRequestTemp: "+err.Error(), c)
		return
	}

	// approve params
	card := &pelecard.PeleCard{}
	if err := card.Init(); err != nil {
		onError("Init"+err.Error(), c)
		return
	}

	var msg map[string]interface{}
	if err, msg = card.GetTransaction(form.PelecardTransactionId); err != nil {
		onError("GetTransaction"+err.Error(), c)
		return
	}

	var response = types.PaymentResponse{}
	body, _ := json.Marshal(msg)
	json.Unmarshal(body, &response)
	response.UserKey = form.UserKey
	// update DB
	if err = db.UpdateRequest(response); err != nil {
		onError("UpdateRequest"+err.Error(), c)
		return
	}
	// real validation
	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		onError("LoadRequest"+err.Error(), c)
		return
	}

	if err := card.Init(); err != nil {
		onError("Init"+err.Error(), c)
		return
	}
	card.ConfirmationKey = form.ConfirmationKey
	card.UserKey = request.UserKey
	card.TotalX100 = fmt.Sprintf("%d", int(request.Price*100.00))
	var valid bool
	if valid, err = card.ValidateByUniqueKey(); err != nil {
		onError("ValidateByUniqueKey"+err.Error(), c)
		return
	}
	if !valid {
		onError("Confirmation error", c)
		return
	}

	// redirect to GoodURL
	v, _ := query.Values(response)
	onSuccess(request.GoodURL, v.Encode(), c)
}

func ErrorPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		onError(err.Error(), c)
		return
	}

	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		onError(err.Error(), c)
		return
	}
	onRedirect(request.ErrorURL, pelecard.GetMessage(form.PelecardStatusCode), c)
}

func CancelPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		onError(err.Error(), c)
		return
	}

	var request = types.PaymentRequest{}
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		onError(err.Error(), c)
		return
	}
	onRedirect(request.ErrorURL, "", c)
}

func onError(err string, c *gin.Context) {
	html := fmt.Sprintf("<html><body><h1 style='color: red;'>Error</h1><code>%s</code><br><pre>%s</pre></body></html>", err, debug.Stack())
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write([]byte(html))
}

func onRedirect(url string, msg string, c *gin.Context) {
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
	c.Writer.Write([]byte(html))
}

func onSuccess(url string, msg string, c *gin.Context) {
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

		target = fmt.Sprintf("%s%ssuccess=%s", url, q, msg)
	}
	html := "<script>window.location = '" + target + "';</script>"
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Write([]byte(html))
}
