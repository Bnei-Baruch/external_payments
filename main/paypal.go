package main

import (
	"external_payments/db"
	"external_payments/types"
	"github.com/gin-gonic/gin"
	"net/http"
)

func ConfirmPaypal(c *gin.Context) {
	var err error
	request := types.PaypalRegister{}
	if err = c.ShouldBindJSON(&request); err != nil { // Bind by JSON (post)
		if err = c.ShouldBindQuery(&request); err != nil { // Bind by Query String (get)
			onError("Bind "+err.Error(), c)
			return
		}
	}
	// Store request into DB
	db.StorePaypal(request)
	c.Status(http.StatusOK)
}
