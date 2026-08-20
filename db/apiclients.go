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
		SELECT name, organization, token_sha256
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
		VALUES (?, ?, ?, NULLIF(?, ''))
	`, name, TokenHash(token), organization, notes)
	if err != nil {
		return 0, fmt.Errorf("insert api client: %w", err)
	}
	return res.LastInsertId()
}

// APIClientRow is a client as shown by -listkeys. The token is not part of it;
// only its hash is stored.
type APIClientRow struct {
	ID           int64   `db:"id"`
	Name         string  `db:"name"`
	Organization string  `db:"organization"`
	Enabled      bool    `db:"enabled"`
	CreatedAt    string  `db:"created_at"`
	LastUsedAt   *string `db:"last_used_at"`
	Notes        string  `db:"notes"`
}

// ListAPIClients returns every client, revoked ones included.
func ListAPIClients() (rows []APIClientRow, err error) {
	err = db.Select(&rows, `
		SELECT id, name, organization, enabled,
		       created_at, last_used_at, COALESCE(notes, '') AS notes
		FROM civicrm_bb_ext_api_clients
		ORDER BY id
	`)
	return
}

// RevokeAPIClient disables a client. The row is kept so the audit trail
// survives; deleting it would lose who held the key and when it was issued.
func RevokeAPIClient(id int64) (bool, error) {
	res, err := db.Exec(`UPDATE civicrm_bb_ext_api_clients SET enabled = 0 WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
