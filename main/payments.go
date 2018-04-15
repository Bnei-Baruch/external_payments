package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/location"
	"github.com/gin-gonic/gin"
	"github.com/google/go-querystring/query"

	"github.com/gshilin/external_payments/db"
	"github.com/gshilin/external_payments/pelecard"
	"github.com/gshilin/external_payments/types"
)

func NewPayment(c *gin.Context) {
	var err error
	request := types.PaymentRequest{}
	if err = c.BindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBind(&request); err != nil { // Bind by Query String (get)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	fmt.Printf("Request: %#v\n", request)
	if errFound, errors := validateStruct(request); errFound {
		c.JSON(http.StatusBadRequest, gin.H{"error": strings.Join(errors, "\n")})
		return
	}

	// Store request into DB
	if _, err = db.StoreRequest(request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	currency := 1 // ILS
	switch request.Currency {
	case "USD":
		currency = 2
	case "EUR":
		currency = 978
	}

	url := location.Get(c)
	goodUrl := fmt.Sprintf("%s://%s/payments/good", url.Scheme, url.Host)
	errorUrl := fmt.Sprintf("%s://%s/payments/error", url.Scheme, url.Host)
	cancelUrl := fmt.Sprintf("%s://%s/payments/cancel", url.Scheme, url.Host)

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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err, url := card.GetRedirectUrl(); err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
	} else {
		c.JSON(http.StatusOK, gin.H{"url": url})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// approve params
	card := &pelecard.PeleCard{}
	if err := card.Init(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var msg map[string]interface{}
	if err, msg = card.GetTransaction(form.PelecardTransactionId); err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
		return
	}

	var response = types.PaymentResponse{}
	body, _ := json.Marshal(msg)
	json.Unmarshal(body, &response)
	response.UserKey = form.UserKey
	// update DB
	if err = db.UpdateRequest(response); err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
		return
	}
	// real validation
	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
		return
	}

	if err := card.Init(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	card.ConfirmationKey = form.ConfirmationKey
	card.UserKey = request.UserKey
	card.TotalX100 = fmt.Sprintf("%d", int(request.Price*100.00))
	var valid bool
	if valid, err = card.ValidateByUniqueKey(); err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
		return
	}
	if !valid {
		c.JSON(http.StatusNotAcceptable, gin.H{"error": "Confirmation error"})
		return
	}

	// redirect to GoodURL
	v, _ := query.Values(response)
	var q string
	if strings.ContainsRune(request.GoodURL, '?') {
		q = "&"
	} else {
		q = "?"
	}
	goodUrl := fmt.Sprintf("%s%s%s", request.GoodURL, q, v.Encode())
	c.Redirect(http.StatusCreated, goodUrl)
}

func ErrorPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var request types.PaymentRequest
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
		return
	}
	var q string
	if strings.ContainsRune(request.GoodURL, '?') {
		q = "&"
	} else {
		q = "?"
	}
	errorURL := fmt.Sprintf("%s%s%s", request.ErrorURL, q,
		"error="+pelecard.GetMessage(form.PelecardStatusCode))
	c.Redirect(http.StatusCreated, errorURL)
}

func CancelPayment(c *gin.Context) {
	var err error

	form := loadPeleCardForm(c)
	if err = db.UpdateRequestTemp(form.UserKey, form); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var request = types.PaymentRequest{}
	if err = db.LoadRequest(form.UserKey, &request); err != nil {
		c.JSON(http.StatusNotAcceptable, gin.H{"error": err.Error()})
		return
	}
	c.Redirect(http.StatusCreated, request.CancelURL)
}

// POST curl --url http://localhost:3001/payments/new -H "Content-Type: applcation/json" -d '{"org":"bb","sku":"123123123","vat":"y","name":"Gregory Shilin","price":10.21,"currency":"USD","installments":1,"details":"test action","email":"gshilin@gmail.com","street":"street","city":"city","country":"country","language":"EN","reference":"ex-123123","userKey":"123123","goodURL":"https://example.com/goodURL","errorURL":"https://example.com/errorURL","cancelURL":"https://example.com/cancelURL"}'
// GET curl 'http://localhost:3001/payments/new?org=bb&sku=123123123&vat=y&name=Gregory%32Shilin&currency=USD&installments=1&details=test%32action&email=gshilin@gmail.com&street=street&city=city&country=country&language=EN&reference=ex123123&goodURL=https://example.com/goodURL&errorURL=https://example.com/errorURL&cancelURL=https://example.com/cancelURL&price=10.22&userKey=123111'