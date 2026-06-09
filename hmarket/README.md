# HMarket Integration

WooCommerce webhook integration for HMarket orders. Stores customers and purchase activity, tracks subscription and blacklist status changes.

## Authentication

`/hmarket/export`, `/hmarket/subscription-status`, and `/hmarket/blacklist` require a Bearer token:

```
Authorization: Bearer <HMARKET_API_TOKEN>
```

`HMARKET_API_TOKEN` is set in the server `.env`. Requests without a valid token receive `401 Unauthorized`.

`/hmarket/webhook`, `/hmarket/hw1`, and `/hmarket/form` do not require this token. `/hmarket/hw1` verifies the WooCommerce webhook signature via `X-Wc-Webhook-Signature` and `HMARKET_SECRET`.

## Endpoints

### `POST /hmarket/form`
Accepts Elementor Pro webhook submissions from landing page forms. No authentication required.

**Fields** (URL-encoded body, Hebrew labels auto-mapped to English IDs):

| Form field | Elementor Field ID | Destination |
|------------|--------------------|-------------|
| Name | `name` | `hmarket_users.first_name` + `last_name` (split by first space) |
| Email | `email` | `hmarket_users.email` |
| Phone | `phone` | `hmarket_users.phone` / `uniq_phone` |
| Event | `event` | `hmarket_activities.name` |
| Source | `source` | `hmarket_activities.source` |

- Requires at least **phone or email**. Returns `400` if both absent.
- User deduped by `uniq_phone` if phone present, otherwise by `email`.
- Always creates one activity row.
- Hebrew field labels (`שם`, `אימייל`, `טלפון`) are auto-mapped via `hebrewAliases`. Elementor uses the label as webhook key when a label is set; use placeholder (not label) for display text to avoid this, or keep labels in the alias map.

### `POST /hmarket/webhook`
Logs all incoming headers and raw body to `/tmp/hmarket.log`. Used for inspection/debugging of WooCommerce webhook payloads.

### `POST /hmarket/hw1`
Processes a WooCommerce order JSON (sent by WooCommerce webhook).

Orders with `status != "completed"` are ignored (returns `{"status":"ignored"}`).

- **User**: created from `billing` fields. If `phone` is present, normalized to international format (`0` prefix → `972`) and used as dedup key (`uniq_phone`). Existing user found by `uniq_phone` is updated (phone and uniq_phone are immutable after creation). `blacklisted` is preserved on update.
- **Activities**: one row per `line_items` entry, with `source` taken from `X-Wc-Webhook-Source` header.
- **Subscription**: determined from `meta_data` — key `cf_extra_consent` or `_cf_extra_consent`, value `yes` = subscribed. If absent or `no` = not subscribed.
- **Subscription history**: recorded when (a) new user is created with subscription `yes` — logged as "new subscriber via \<source\>", or (b) existing user's subscription status changes.

### `GET /hmarket/export`
Downloads an Excel file with one row per activity:

| Column | Source |
|--------|--------|
| ID | `hmarket_users.id` |
| First Name | `billing.first_name` |
| Last Name | `billing.last_name` |
| Phone | raw phone |
| Uniq Phone | normalized international phone |
| Email | `billing.email` |
| Company | `billing.company` |
| City | `billing.city` |
| Country | `billing.country` |
| Subscribed | boolean |
| Blacklisted | boolean |
| Source | `X-Wc-Webhook-Source` |
| Product Name | `line_items[].name` |
| Product ID | `line_items[].product_id` |
| SKU | `line_items[].sku` |
| Created At | `date_created` |

### `GET /hmarket/subscription-status`
Downloads an Excel file with one row per user. History column contains all subscription/blacklist changes, one per line:

```
2026-06-03 07:33:57 | blacklist | true | manually blacklisted
2026-06-03 08:00:00 | subscription | false | subscription changed to false due to shop.example.com
```

### `POST /hmarket/blacklist`
Updates a user's blacklisted flag and records a history entry.

Request body:
```json
{
  "user_id": 2,
  "description": "reason for change",
  "blacklist": true
}
```

Returns `404` if user not found.

## Phone Normalization

1. Strip all non-digit characters
2. If result starts with `0`, replace with `972`
3. Empty phone → `NULL` in DB (not used for dedup; new user created each time)

## Database Tables

- `hmarket_users` — one row per customer, deduped by `uniq_phone` (or `email` when phone absent)
- `hmarket_activities` — one row per line item per order
- `hmarket_subscription_history` — audit log of subscription and blacklist changes; `change_type` is `subscription` or `blacklist`
