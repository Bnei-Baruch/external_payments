package utils

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"external_payments/db"
)

func ctxWithClient(client *db.APIClient) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/emv/charge", nil)
	if client != nil {
		c.Set(APIClientKey, *client)
	}
	return c
}

// The two plugin generations have to coexist: sites update at their own pace.
func TestResolveOrganizationOldPluginNoToken(t *testing.T) {
	c := ctxWithClient(nil)
	if got := ResolveOrganization(c, "meshp18"); got != "meshp18" {
		t.Errorf("body value should be used when unauthenticated, got %q", got)
	}
}

func TestResolveOrganizationNewPluginNoBodyField(t *testing.T) {
	c := ctxWithClient(&db.APIClient{Name: "1family", Organization: "meshp18"})
	if got := ResolveOrganization(c, ""); got != "meshp18" {
		t.Errorf("key value should fill in an absent body field, got %q", got)
	}
}

func TestResolveOrganizationKeyBeatsBody(t *testing.T) {
	c := ctxWithClient(&db.APIClient{Name: "1family", Organization: "meshp18"})
	if got := ResolveOrganization(c, "ben2"); got != "meshp18" {
		t.Errorf("key must win over a caller-supplied value, got %q", got)
	}
}

// A client row without an organization must not blank out a working request.
func TestResolveOrganizationClientWithoutOrg(t *testing.T) {
	c := ctxWithClient(&db.APIClient{Name: "internal"})
	if got := ResolveOrganization(c, "ben2"); got != "ben2" {
		t.Errorf("body value should survive a client with no organization, got %q", got)
	}
}

func TestResolveOrganizationNeitherSide(t *testing.T) {
	c := ctxWithClient(nil)
	if got := ResolveOrganization(c, ""); got != "" {
		t.Errorf("nothing to resolve should stay empty so validation rejects it, got %q", got)
	}
}
