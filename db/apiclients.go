package db

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// APIClient is one row of civicrm_bb_ext_api_clients, minus the secret. The
// token itself is never stored — only its SHA-256 — so a lost key is reissued,
// not recovered. Keys are high-entropy, so a fast hash is appropriate here; a
// password KDF would only add latency to the payment path.
type APIClient struct {
	Name         string `db:"name"`
	Organization string `db:"organization"`
	TokenSHA256  string `db:"token_sha256"`
}

var (
	apiClientsMu sync.RWMutex
	apiClients   = map[string]APIClient{}
)

// LoadAPIClients reads the enabled clients into memory. Called at startup, so
// there is no database round trip on the request path.
//
// The consequence is that adding or revoking a key needs a restart. If that
// becomes awkward, call this again on a ticker — the map is swapped under a
// write lock, so it is safe to do while serving.
func LoadAPIClients() (int, error) {
	var rows []APIClient
	err := db.Select(&rows, `
		SELECT name, COALESCE(organization, '') AS organization, token_sha256
		FROM civicrm_bb_ext_api_clients
		WHERE enabled = 1
	`)
	if err != nil {
		return 0, err
	}

	next := make(map[string]APIClient, len(rows))
	for _, r := range rows {
		next[r.TokenSHA256] = r
	}

	apiClientsMu.Lock()
	apiClients = next
	apiClientsMu.Unlock()

	return len(next), nil
}

// TokenHash is the value stored in token_sha256.
func TokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// LookupAPIClient resolves a presented token. Comparison is a map lookup on the
// hash, so it does not leak the token through timing.
func LookupAPIClient(token string) (APIClient, bool) {
	if token == "" {
		return APIClient{}, false
	}
	apiClientsMu.RLock()
	defer apiClientsMu.RUnlock()
	c, ok := apiClients[TokenHash(token)]
	return c, ok
}

// APIClientCount reports how many clients are loaded, for the startup log.
func APIClientCount() int {
	apiClientsMu.RLock()
	defer apiClientsMu.RUnlock()
	return len(apiClients)
}

// CreateAPIClient stores a new client and returns the row id. The caller keeps
// the token; it cannot be read back afterwards.
func CreateAPIClient(name, organization, token, notes string) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO civicrm_bb_ext_api_clients (name, token_sha256, organization, notes)
		VALUES (?, ?, NULLIF(?, ''), NULLIF(?, ''))
	`, name, TokenHash(token), organization, notes)
	if err != nil {
		return 0, fmt.Errorf("insert api client: %w", err)
	}
	return res.LastInsertId()
}
