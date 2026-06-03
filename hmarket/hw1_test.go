package hmarket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"external_payments/types"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"0538268898",    "972538268898"},
		{"053-826-8898",  "972538268898"},
		{"+972538268898", "972538268898"},
		{"972538268898",  "972538268898"},
		{"",              ""},
		{"abc",           ""},
	}
	for _, tc := range cases {
		got := normalizePhone(tc.in)
		if got != tc.want {
			t.Errorf("normalizePhone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestStrPtr(t *testing.T) {
	if strPtr("") != nil {
		t.Error("strPtr(\"\") should return nil")
	}
	s := strPtr("hello")
	if s == nil || *s != "hello" {
		t.Error("strPtr(\"hello\") should return pointer to \"hello\"")
	}
}

func TestExtractSubscription(t *testing.T) {
	cases := []struct {
		meta []wcMetaData
		want bool
	}{
		{[]wcMetaData{{"_cf_extra_consent", "yes"}}, true},
		{[]wcMetaData{{"cf_extra_consent", "yes"}}, true},
		{[]wcMetaData{{"_cf_extra_consent", "no"}}, false},
		{[]wcMetaData{{"other_key", "yes"}}, false},
		{[]wcMetaData{}, false},
	}
	for _, tc := range cases {
		got := extractSubscription(tc.meta)
		if got != tc.want {
			t.Errorf("extractSubscription(%v) = %v, want %v", tc.meta, got, tc.want)
		}
	}
}

const sampleOrder = `{
	"status": "completed",
	"date_created": "2026-06-02T13:17:13",
	"billing": {
		"first_name": "Sarah",
		"last_name":  "Cohen",
		"company":    "",
		"address_1":  "Main St 1",
		"address_2":  "",
		"city":       "Tel Aviv",
		"country":    "IL",
		"email":      "sarah@example.com",
		"phone":      "0538268898"
	},
	"line_items": [
		{"name": "Book A", "product_id": 100, "sku": "SKU-1"},
		{"name": "Book B", "product_id": 101, "sku": ""}
	],
	"meta_data": []
}`

const sampleOrderSubscribed = `{
	"status": "completed",
	"date_created": "2026-06-02T13:17:13",
	"billing": {
		"first_name": "Sarah",
		"last_name":  "Cohen",
		"company":    "",
		"address_1":  "Main St 1",
		"address_2":  "",
		"city":       "Tel Aviv",
		"country":    "IL",
		"email":      "sarah@example.com",
		"phone":      "0538268898"
	},
	"line_items": [
		{"name": "Book A", "product_id": 100, "sku": "SKU-1"}
	],
	"meta_data": [{"key": "_cf_extra_consent", "value": "yes"}]
}`

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/hmarket/hw1", HW1)
	return r
}

func TestHW1_Success(t *testing.T) {
	dbUpsertUser = func(u types.HMarketUser) (int64, bool, bool, bool, error) {
		if u.UniqPhone == nil || *u.UniqPhone != "972538268898" {
			t.Errorf("expected uniq_phone 972538268898, got %v", u.UniqPhone)
		}
		if u.Phone == nil || *u.Phone != "0538268898" {
			t.Errorf("expected raw phone 0538268898, got %v", u.Phone)
		}
		return 42, false, false, false, nil
	}

	activitiesCreated := 0
	dbCreateActivity = func(a types.HMarketActivity) error {
		activitiesCreated++
		if a.UserID != 42 {
			t.Errorf("expected user_id 42, got %d", a.UserID)
		}
		if a.CreatedAt != "2026-06-02 13:17:13" {
			t.Errorf("expected createdAt 2026-06-02 13:17:13, got %s", a.CreatedAt)
		}
		return nil
	}

	dbCreateSubHistory = func(h types.HMarketSubscriptionHistory) error {
		t.Error("subscription history should not be created when sub unchanged")
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/hmarket/hw1",
		strings.NewReader(sampleOrder))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Wc-Webhook-Source", "365tfilot.co.il")

	w := httptest.NewRecorder()
	newTestRouter().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if activitiesCreated != 2 {
		t.Errorf("expected 2 activities, got %d", activitiesCreated)
	}
}

func TestHW1_NewSubscriber(t *testing.T) {
	dbUpsertUser = func(u types.HMarketUser) (int64, bool, bool, bool, error) {
		if !u.Subscribed {
			t.Error("expected Subscribed=true from meta_data")
		}
		return 55, true, false, false, nil // isNew=true
	}
	dbCreateActivity = func(a types.HMarketActivity) error { return nil }

	subHistoryCalled := false
	dbCreateSubHistory = func(h types.HMarketSubscriptionHistory) error {
		subHistoryCalled = true
		if h.UserID != 55 {
			t.Errorf("expected user_id 55, got %d", h.UserID)
		}
		if !h.Status {
			t.Error("expected status true for new subscriber")
		}
		if !strings.Contains(h.Description, "new subscriber") {
			t.Errorf("expected 'new subscriber' in description, got: %s", h.Description)
		}
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/hmarket/hw1",
		strings.NewReader(sampleOrderSubscribed))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Wc-Webhook-Source", "365tfilot.co.il")

	w := httptest.NewRecorder()
	newTestRouter().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !subHistoryCalled {
		t.Error("subscription history was not created for new subscriber")
	}
}

func TestHW1_SubscriptionChanged(t *testing.T) {
	dbUpsertUser = func(u types.HMarketUser) (int64, bool, bool, bool, error) {
		return 7, false, true, false, nil // isNew=false, subChanged=true
	}
	dbCreateActivity = func(a types.HMarketActivity) error { return nil }

	subHistoryCalled := false
	dbCreateSubHistory = func(h types.HMarketSubscriptionHistory) error {
		subHistoryCalled = true
		if h.UserID != 7 {
			t.Errorf("expected user_id 7, got %d", h.UserID)
		}
		if h.Status != false {
			t.Error("expected status false")
		}
		if !strings.Contains(h.Description, "365tfilot.co.il") {
			t.Errorf("description should contain source, got: %s", h.Description)
		}
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/hmarket/hw1",
		strings.NewReader(sampleOrder))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Wc-Webhook-Source", "365tfilot.co.il")

	w := httptest.NewRecorder()
	newTestRouter().ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !subHistoryCalled {
		t.Error("subscription history was not created")
	}
}

func TestHW1_BadJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/hmarket/hw1",
		strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	newTestRouter().ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
