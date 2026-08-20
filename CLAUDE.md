
# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

---

## Project Overview

Payment processing system built with Go integrating multiple payment gateways (primarily Pelecard) with CiviCRM and Priority ERP. Multiple standalone services handle different aspects of the payment lifecycle.

### Architecture

Services handle payment data flow between:
- **CiviCRM** (contact/contribution management system)
- **Pelecard** (payment gateway for credit card processing)
- **Priority ERP** (Israeli business management system)
- **external_payments** (REST API for payment processing)

### Service Descriptions

- **external_payments/** - Main REST API service (Gin framework). Handles payment requests, confirmations, and various payment types (regular, token-based, EMV, PayPal).
- **4priority/** - HTTP server that receives payment data and submits it to Priority ERP via REST API.
- **bb2prio/** - Batch processor: reads completed contributions from CiviCRM and forwards to Priority ERP.
- **ext2prio/** - Like bb2prio but for external payment contributions.
- **ext2fix/** - Reconciles stuck Pelecard payments via `CheckGoodParamX`. Excludes PayPal records (paypal_order_id IS NOT NULL). Skips records < 10 min old to avoid race with active payments.
- **pp2prio/** - Reads captured PayPal transactions from `civicrm_bb_ext_paypal` and forwards to Priority. Filters by `PAYPAL_ENV` to isolate sandbox from live.
- **pp2fix/** - Reconciles stuck PayPal payments by calling PayPal CaptureOrder. Skips records < 3h old with CREATED/APPROVED status.
- **prioRecurr2Civi/** - Fetches recurring Pelecard transactions, matches with Priority records, creates contributions in CiviCRM.
- **bb2fix/** - Correction/fixing service for BB payments.
- **woocommerce-bb/** - WooCommerce plugin with two gateways (EMV + PayPal) routing through external_payments.

### Build

```bash
# external_payments (requires GOEXPERIMENT=jsonv2)
cd external_payments
GOEXPERIMENT=jsonv2 CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o external_payments main/*
# Or simply: make build

# Other services
cd pp2prio
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o pp2prio pp2prio.go && upx -9 pp2prio
```

### Deployment

```bash
scp external_payments bb2priority:/sites/bb2priority/external_payments
# Then on server:
sudo systemctl stop external-payments && sudo systemctl start external-payments
```

### Configuration

All services use `.env` files (godotenv). Key variables:

**CiviCRM:** `CIVI_HOST`, `CIVI_DBNAME`, `CIVI_USER`, `CIVI_PASSWORD`, `CIVI_PROTOCOL`

**Priority ERP:** `PRIO_HOST`, `PRIO_API_URL`, `PRIO_API_ORG`, `PRIO_API_USER`, `PRIO_API_PASSWORD`

**Pelecard:** `PELECARD_TERMINAL`, `PELECARD_USER`, `PELECARD_PASSWORD` (multiple terminal configs: `ben2_PELECARD_TERMINAL`, `meshp18_PELECARD_TERMINAL`)

**external_payments:** `ENV` (production/development), `EXT_PORT` (default 8080)

**PayPal:** `PAYPAL_CLIENT_ID`, `PAYPAL_CLIENT_SECRET`, `PAYPAL_ENV` (sandbox/live)

### external_payments Payment Flow

The caller (e.g. WooCommerce plugin) supplies `GoodURL`, `ErrorURL`, `CancelURL` pointing back to itself. external_payments stores these, substitutes its own internal URLs when talking to Pelecard/PayPal. Upon return from the payment provider, external_payments updates its DB and redirects the browser to the original caller-supplied URLs.

Key facts:
- Callers must supply real callback URLs they own and handle.
- external_payments never calls GoodURL/ErrorURL/CancelURL server-side — browser redirect only.
- `civicrm_bb_ext_requests` holds flow state (`status`, `pstatus`, `paypal_order_id`, `paypal_env`).
- `civicrm_bb_ext_paypal` holds captured PayPal transactions; pp2prio reads these to forward to Priority.
- `civicrm_bb_ext_payment_responses` holds Pelecard responses; ext2prio reads these.
- PayPal records never appear in `civicrm_bb_ext_payment_responses` — ext2prio ignores them safely.

### API Endpoints (external_payments)

**EMV (Pelecard):**
- `/emv/new` - Initiate EMV payment (POST JSON, returns `{"url": "..."}`)
- `/emv/good`, `/emv/error`, `/emv/cancel` - Pelecard callbacks

**PayPal:**
- `/paypal/new` - Initiate PayPal order (POST JSON, returns `{"url": "..."}`)
- `/paypal/good` - Return after approval; captures order, stores result, redirects to GoodURL
- `/paypal/error`, `/paypal/cancel` - Callbacks; redirect to ErrorURL/CancelURL

**Token-based:** `/token/charge`, `/token/refund`

**Other:** `/payments/new`, `/payments/confirm`, `/renew/renew-card`, `/hmarket/*`, `/projects/:language/:project_name/counter`

### API client authentication

Guarded routes require `Authorization: Bearer <token>`. Two sources, checked in
this order:

1. **`civicrm_bb_ext_api_clients`** — third-party callers (WooCommerce sites,
   VH). The row carries an `organization`, which is how the caller stops being
   able to choose it in the request body. Loaded once at startup, so the request
   path does no database work; a new or revoked key therefore needs a restart.
   Only the SHA-256 of the token is stored.
2. **`INTERNAL_API_TOKEN`** — a single shared secret for our own services.
   4priority uses it. They have no organization of their own, so a row each
   would be bookkeeping without benefit. Rotating it means editing `.env` and
   restarting both this service and every service of ours that calls it.

Two middlewares, and the difference matters:

- `utils.RequireAPIClient()` — always enforces. 401 without a valid token,
  whatever is configured. A route guarded with this is never open.
- `utils.ObserveAPIClient()` — resolves a token if one is present, lets every
  request through either way, and logs unauthenticated callers as
  `AUTH OBSERVE`.

Observe mode is how a route with callers we cannot enumerate gets guarded:
deploy in observe mode, read the log to find every caller, issue their keys,
then switch the route to require. Enforcing first takes payments down for
whoever we forgot. **A route left in observe mode is unprotected** — it is a
deployment step, not a resting state.

Management is via the binary; `-h` lists the commands. It deliberately does not
explain any of the above, since the binary gets copied between hosts.

Organization resolution supports two generations of a caller at once, because
WordPress sites update at their own pace:

| Caller | Sends | Organization taken from |
|---|---|---|
| old plugin | `Organization` in body, no token | the body |
| new plugin | token, no `Organization` | the key |
| either, misconfigured | both, disagreeing | the key, logged as `ORG MISMATCH` |

`utils.ResolveOrganization` must be called after binding and **before**
validation — `Organization` is a required field, so a request from the new
plugin fails validation unless the value is filled in first.

Route status:

| Route | Mode |
|---|---|
| `POST /payments/transaction` | require |
| `POST /token/charge`, `/token/chargex` | observe |
| `POST /emv/charge` | observe |
| `POST /paypal/charge` | observe |

### DB Tables

- `civicrm_bb_ext_requests` — one row per payment attempt; tracks status/pstatus/paypal_order_id/paypal_env
- `civicrm_bb_ext_paypal` — captured PayPal payments; status='new' → pp2prio picks up and marks 'processed'
- `civicrm_bb_ext_payment_responses` — Pelecard transaction results
