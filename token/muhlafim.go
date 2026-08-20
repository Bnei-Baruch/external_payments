package token

import (
	"encoding/json/v2"
	"net/http"

	"github.com/gin-gonic/gin"

	"external_payments/pelecard"
	"external_payments/types"
	"external_payments/utils"
)

// Muhlafim reports the card replacements Pelecard recorded for the recurring
// terminal in a date window, keyed by the token being replaced.
//
// This lives here rather than in the caller because the tokens it describes are
// charged through /token/charge: the terminal they sit on is this service's
// business, and a caller asking about replacements should not have to hold
// Pelecard credentials to do it.
func Muhlafim(c *gin.Context) {
	var request types.MuhlafimRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.ErrorJson(http.StatusBadRequest, "Bind "+err.Error(), c)
		return
	}

	request.Organization = utils.ResolveOrganization(c, request.Organization)
	if request.Organization == "" {
		utils.ErrorJson(http.StatusBadRequest, "Organization cannot be blank", c)
		return
	}
	if request.StartDate == "" || request.EndDate == "" {
		utils.ErrorJson(http.StatusBadRequest, "StartDate and EndDate cannot be blank", c)
		return
	}

	// Recurrent: replacements are only meaningful for the terminal the
	// recurring tokens live on, which is the one /token/charge charges.
	card := &pelecard.PeleCard{}
	if err := card.Init(request.Organization, types.Recurrent, true); err != nil {
		utils.ErrorJson(http.StatusBadGateway, "Init "+err.Error(), c)
		return
	}

	err, entries := card.FetchMuhlafim(request.StartDate, request.EndDate)
	if err != nil {
		utils.LogMessage("Muhlafim: " + err.Error())
		utils.ErrorJson(http.StatusBadGateway, "FetchMuhlafim "+err.Error(), c)
		return
	}

	body, _ := json.Marshal(entries)
	c.Writer.WriteHeader(http.StatusOK)
	_, _ = c.Writer.Write(body)
}
