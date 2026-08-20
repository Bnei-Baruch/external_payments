package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strconv"

	"external_payments/db"
)

const usage = `external_payments — payment gateway

Run with no arguments to start the server.

API tokens
  Callers of guarded routes present:  Authorization: Bearer <token>

  Two kinds exist:

    Our own services (4priority, and anything else we deploy) share the
    single INTERNAL_API_TOKEN from .env. They have no organization of
    their own, so a row each would be bookkeeping without benefit.

    Third-party callers (WooCommerce sites, VH) get a row in
    civicrm_bb_ext_api_clients. The row carries the organization, so the
    caller no longer chooses it in the request body.

  Only the SHA-256 of a token is stored. A token that is lost cannot be
  read back — issue a new one and revoke the old.

Commands
  -createkey <name> [organization] [notes]
        Mint a token and print it once. Store it immediately.
        e.g.  ./external_payments -createkey 1family-woo meshp18 "wp plugin"

  -listkeys
        Show every client, revoked included. Never shows tokens.

  -revokekey <id>
        Disable a client. The row is kept so you keep the audit trail of
        who held the key and when it was issued.

  -h, -help
        This text.

After any change
  Clients are loaded once at startup, so the request path does no
  database work. A new or revoked key takes effect on restart:

      sudo systemctl restart external-payments

  The startup line reports what is active, e.g.
      api clients: 3 loaded, internal token set: true — guarded routes enforced

  With no clients and no INTERNAL_API_TOKEN, guarded routes stay OPEN and
  log every caller. That is deliberate: it lets you see who is really
  calling before you switch enforcement on.

Rotating INTERNAL_API_TOKEN
  Edit .env, then restart both this service and any of ours that call it
  (4priority reads the same variable).
`

// runCommand handles the CLI. Anything that touches the table connects to the
// database itself, since the server only connects in production.
func runCommand(args []string) {
	switch args[0] {
	case "-h", "-help", "--help", "help":
		fmt.Print(usage)

	case "-createkey":
		withDB(func() { createKey(args[1:]) })

	case "-listkeys":
		withDB(listKeys)

	case "-revokekey":
		withDB(func() { revokeKey(args[1:]) })

	default:
		fmt.Printf("unknown option %q\n\n", args[0])
		fmt.Print(usage)
		os.Exit(2)
	}
}

func withDB(fn func()) {
	if err := db.Connect(); err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Disconnect()
	fn()
}

// createKey mints a client token, prints it once and exits. The token is not
// recoverable afterwards — only its hash is stored.
func createKey(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: external_payments -createkey <name> [organization] [notes]")
		os.Exit(2)
	}
	var organization, notes string
	if len(args) > 1 {
		organization = args[1]
	}
	if len(args) > 2 {
		notes = args[2]
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		log.Fatalf("generate token: %v", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	id, err := db.CreateAPIClient(args[0], organization, token, notes)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	fmt.Printf("id            %d\n", id)
	fmt.Printf("name          %s\n", args[0])
	if organization != "" {
		fmt.Printf("organization  %s\n", organization)
	}
	fmt.Printf("token         %s\n", token)
	fmt.Println("\nStore it now — only the hash is kept.")
	fmt.Println("Restart the service to load it: sudo systemctl restart external-payments")
}

func listKeys() {
	rows, err := db.ListAPIClients()
	if err != nil {
		log.Fatalf("list clients: %v", err)
	}
	if len(rows) == 0 {
		fmt.Println("no clients. Guarded routes fall back to INTERNAL_API_TOKEN,")
		fmt.Println("and stay open if that is unset too.")
		return
	}
	fmt.Printf("%-4s %-22s %-12s %-9s %-20s %s\n",
		"id", "name", "organization", "state", "created", "last used")
	for _, r := range rows {
		state := "revoked"
		if r.Enabled {
			state = "enabled"
		}
		last := "never"
		if r.LastUsedAt != nil {
			last = *r.LastUsedAt
		}
		fmt.Printf("%-4d %-22s %-12s %-9s %-20s %s\n",
			r.ID, r.Name, r.Organization, state, r.CreatedAt, last)
	}
}

func revokeKey(args []string) {
	if len(args) < 1 {
		fmt.Println("usage: external_payments -revokekey <id>    (see -listkeys)")
		os.Exit(2)
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		log.Fatalf("id must be a number: %v", err)
	}
	ok, err := db.RevokeAPIClient(id)
	if err != nil {
		log.Fatalf("revoke: %v", err)
	}
	if !ok {
		fmt.Printf("no client with id %d\n", id)
		os.Exit(1)
	}
	fmt.Printf("client %d revoked\n", id)
	fmt.Println("Takes effect on restart: sudo systemctl restart external-payments")
}
