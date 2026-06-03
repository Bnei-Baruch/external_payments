# HMarket Integration

WooCommerce webhook integration for HMarket orders. Stores customers and purchase activity, tracks subscription and blacklist status changes.

## Endpoints

### `POST /hmarket/webhook`
Logs all incoming headers and raw body to `/tmp/hmarket.log`. Used for inspection/debugging of WooCommerce webhook payloads.

### `POST /hmarket/hw1`
Processes a WooCommerce order JSON (sent by WooCommerce webhook).

Orders with `status != "completed"` are ignored (returns `{"status":"ignored"}`).

- **User**: created from `billing` fields. If `phone` is present, normalized to international format (`0` prefix → `972`) and used as dedup key (`uniq_phone`). Existing user found by `uniq_phone` is updated (phone and uniq_phone are immutable after creation). `blacklisted` is preserved on update.
- **Activities**: one row per `line_items` entry, with `source` taken from `X-Wc-Webhook-Source` header.
- **Subscription history**: if subscription status changes for an existing user, a `subscription` history record is created.

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

- `hmarket_users` — one row per customer, deduped by `uniq_phone`
- `hmarket_activities` — one row per line item per order
- `hmarket_subscription_history` — audit log of subscription and blacklist changes; `change_type` is `subscription` or `blacklist`
