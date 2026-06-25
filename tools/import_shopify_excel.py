#!/usr/bin/env python3
"""
Send Excel rows as Shopify checkout webhooks to hmarket/shopify endpoint.

Usage:
    SHOPIFY_SECRET=<secret> python3 import_shopify_excel.py <excel_file> [endpoint]

Excel columns (A-K):
    A: Customer email
    B: Product title
    C: Day (YYYY-MM-DD)
    D: First Name
    E: Last Name
    F: Accepts Email Marketing (yes/no)
    G: Address1
    H: Address2
    I: City
    J: Phone (address)
    K: Phone (customer)
"""

import hashlib
import hmac
import base64
import json
import os
import time
import uuid
import sys
import requests
import openpyxl
from datetime import datetime

ENDPOINT = os.getenv("SHOPIFY_ENDPOINT", "https://checkout.kbb1.com/hmarket/shopify")
SHOPIFY_SECRET = os.getenv("SHOPIFY_SECRET", "")
EXCEL_FILE = sys.argv[1] if len(sys.argv) > 1 else "purchases_filtered_no_zero_sales_or_qty.xlsx"
DELAY = 0.3  # seconds between requests


def sign_body(body: bytes) -> str:
    if not SHOPIFY_SECRET:
        return ""
    mac = hmac.new(SHOPIFY_SECRET.encode(), body, hashlib.sha256)
    return base64.b64encode(mac.digest()).decode()


wb = openpyxl.load_workbook(EXCEL_FILE)
ws = wb.active

ok = 0
skipped = 0
errors = 0

for i, row in enumerate(ws.iter_rows(min_row=2, values_only=True), start=2):
    email      = row[0]  # A
    title      = row[1]  # B
    day        = row[2]  # C
    first_name = row[3]  # D
    last_name  = row[4]  # E
    subscribed = str(row[5]).lower() == "yes" if row[5] else False  # F
    address1   = row[6]  # G
    address2   = str(row[7]) if row[7] is not None else ""  # H
    city       = row[8]  # I
    phone_addr = str(row[9]).strip().strip("'\"") if row[9] else ""  # J
    phone_cust = str(row[10]).strip().strip("'\"") if row[10] else ""  # K

    phone = phone_addr or phone_cust

    if not email and not phone:
        skipped += 1
        continue

    if isinstance(day, datetime):
        completed_at = day.strftime("%Y-%m-%dT%H:%M:%SZ")
    elif day:
        try:
            completed_at = datetime.strptime(str(day), "%Y-%m-%d").strftime("%Y-%m-%dT00:00:00Z")
        except Exception:
            completed_at = "2025-01-01T00:00:00Z"
    else:
        completed_at = "2025-01-01T00:00:00Z"

    payload = {
        "cart_token": str(uuid.uuid4()),
        "email": email or "",
        "phone": phone,
        "completed_at": completed_at,
        "buyer_accepts_marketing": subscribed,
        "billing_address": {
            "first_name": first_name or "",
            "last_name": last_name or "",
            "phone": phone,
            "address1": address1 or "",
            "address2": address2,
            "city": city or "",
            "country": "",
        },
        "customer": {
            "email": email or "",
            "first_name": first_name or "",
            "last_name": last_name or "",
            "phone": phone,
            "default_address": {
                "first_name": first_name or "",
                "last_name": last_name or "",
                "phone": phone,
                "address1": address1 or "",
                "address2": address2,
                "city": city or "",
                "country": "",
            },
        },
        "line_items": [
            {"title": title or "", "product_id": 0, "sku": ""}
        ],
    }

    try:
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        sig = sign_body(body)
        headers = {
            "Content-Type": "application/json",
            "X-Shopify-Shop-Domain": "excel-import",
        }
        if sig:
            headers["X-Shopify-Hmac-Sha256"] = sig

        resp = requests.post(ENDPOINT, data=body, headers=headers, timeout=10)
        if resp.status_code == 200:
            ok += 1
            print(f"row {i}: ok  email={email} {resp.json()}")
        else:
            errors += 1
            print(f"row {i}: ERR {resp.status_code} email={email} body={resp.text[:200]}", file=sys.stderr)
    except Exception as e:
        errors += 1
        print(f"row {i}: EXCEPTION email={email} err={e}", file=sys.stderr)

    time.sleep(DELAY)

print(f"\nDone: ok={ok} skipped={skipped} errors={errors}")
