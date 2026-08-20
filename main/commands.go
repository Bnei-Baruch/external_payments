package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"

	"external_payments/db"
)

// usage is deliberately thin. It says what an operator has to type and
// nothing about how authentication is configured or what happens when it
// is not — that belongs in the repository, not in a binary that gets
// copied between hosts.
const usage = `external_payments

Run with no arguments to start the server.

  -createkey <name> <organization> <prefix> [notes]
        issue a client token
  -listkeys
        list clients
  -revokekey <id>
        disable a client
  -h
        this text

A token is shown once, when issued, and cannot be read back.
Changes take effect on restart.

See the repository for how clients are configured.
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
// validOrganizations mirrors the values= tag on PaymentRequest.Organization in
// types/types.go. A key scoped to anything else could never charge, so it is
// caught here rather than discovered on the first payment.
var validOrganizations = []string{"ben2", "meshp18"}

func createKey(args []string) {
	if len(args) < 3 {
		fmt.Printf("usage: external_payments -createkey <name> <organization> <prefix> [notes]\n")
		fmt.Printf("organization is one of: %s\n", strings.Join(validOrganizations, ", "))
		fmt.Printf("prefix is the caller's reference prefix, e.g. 1fam\n")
		os.Exit(2)
	}
	name, organization, prefix := args[0], args[1], args[2]

	if strings.TrimSpace(prefix) == "" {
		fmt.Println("prefix cannot be blank")
		os.Exit(2)
	}

	if !slices.Contains(validOrganizations, organization) {
		fmt.Printf("unknown organization %q — expected one of: %s\n",
			organization, strings.Join(validOrganizations, ", "))
		os.Exit(2)
	}

	var notes string
	if len(args) > 3 {
		notes = args[3]
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		log.Fatalf("generate token: %v", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	id, err := db.CreateAPIClient(name, organization, prefix, token, notes)
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	fmt.Printf("id            %d\n", id)
	fmt.Printf("name          %s\n", name)
	fmt.Printf("organization  %s\n", organization)
	fmt.Printf("prefix        %s\n", prefix)
	fmt.Printf("token         %s\n", token)
	fmt.Println("\nStore it now — it cannot be read back.")
	fmt.Println("Restart the service to load it.")
}

func listKeys() {
	rows, err := db.ListAPIClients()
	if err != nil {
		log.Fatalf("list clients: %v", err)
	}
	if len(rows) == 0 {
		fmt.Println("no clients")
		return
	}
	fmt.Printf("%-4s %-20s %-12s %-10s %-9s %-20s %s\n",
		"id", "name", "organization", "prefix", "state", "created", "last used")
	for _, r := range rows {
		state := "revoked"
		if r.Enabled {
			state = "enabled"
		}
		last := "never"
		if r.LastUsedAt != nil {
			last = *r.LastUsedAt
		}
		fmt.Printf("%-4d %-20s %-12s %-10s %-9s %-20s %s\n",
			r.ID, r.Name, r.Organization, r.Prefix, state, r.CreatedAt, last)
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
	fmt.Println("Takes effect on restart.")
}
