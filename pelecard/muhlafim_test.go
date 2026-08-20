package pelecard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func muhlafimCard(t *testing.T, handler http.HandlerFunc) (*PeleCard, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	card := &PeleCard{
		Service:  server.URL,
		User:     "user",
		Password: "pass",
		Terminal: "123",
	}
	return card, server.Close
}

func TestFetchMuhlafimKeysByToken(t *testing.T) {
	card, done := muhlafimCard(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"StatusCode":"000","ResultData":[
			{"Token":"tok1","ActionDescription":"חיוב נקלט","NewCardNumber":"1234","NewExpirationDate":"0130"},
			{"Token":"tok2","ActionDescription":"נדחה לא יחויב"}
		]}`))
	})
	defer done()

	err, entries := card.FetchMuhlafim("01/08/2026 00:00", "20/08/2026 00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries["tok1"].NewCardNumber != "1234" {
		t.Errorf("tok1 NewCardNumber = %q", entries["tok1"].NewCardNumber)
	}
	if entries["tok1"].NewExpirationDate != "0130" {
		t.Errorf("tok1 NewExpirationDate = %q", entries["tok1"].NewExpirationDate)
	}
	if entries["tok2"].ActionDescription != "נדחה לא יחויב" {
		t.Errorf("tok2 ActionDescription = %q", entries["tok2"].ActionDescription)
	}
}

// An entry with no token names no card, so there is nothing to act on.
func TestFetchMuhlafimDropsEntriesWithoutToken(t *testing.T) {
	card, done := muhlafimCard(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"StatusCode":"000","ResultData":[
			{"Token":"","NewCardNumber":"1111"},
			{"Token":"tok1","NewCardNumber":"2222"}
		]}`))
	})
	defer done()

	err, entries := card.FetchMuhlafim("01/08/2026 00:00", "20/08/2026 00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d: %v", len(entries), entries)
	}
	if _, ok := entries["tok1"]; !ok {
		t.Error("tok1 should have survived")
	}
}

// A window with no replacements is the normal case, and Pelecard reports it as
// a null ResultData. This used to panic on a bare type assertion.
func TestFetchMuhlafimEmptyWindow(t *testing.T) {
	card, done := muhlafimCard(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"StatusCode":"000","ResultData":null}`))
	})
	defer done()

	err, entries := card.FetchMuhlafim("01/08/2026 00:00", "20/08/2026 00:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("want no entries, got %v", entries)
	}
}

func TestFetchMuhlafimGatewayError(t *testing.T) {
	card, done := muhlafimCard(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"StatusCode":"033","ErrorMessage":"terminal not found"}`))
	})
	defer done()

	err, entries := card.FetchMuhlafim("01/08/2026 00:00", "20/08/2026 00:00")
	if err == nil {
		t.Fatal("want an error")
	}
	if entries != nil {
		t.Errorf("want no entries on error, got %v", entries)
	}
}

// The terminal and credentials come from this service, never from the caller.
func TestFetchMuhlafimSendsTerminalAndWindow(t *testing.T) {
	var got string
	card, done := muhlafimCard(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got = string(body)
		w.Write([]byte(`{"StatusCode":"000","ResultData":[]}`))
	})
	defer done()

	if err, _ := card.FetchMuhlafim("01/08/2026 00:00", "20/08/2026 23:59"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{`"123"`, `"01/08/2026 00:00"`, `"20/08/2026 23:59"`} {
		if !strings.Contains(got, want) {
			t.Errorf("request body %s missing %s", got, want)
		}
	}
}
