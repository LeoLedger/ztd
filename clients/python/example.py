"""
Address Parse API — Python Client Example
=========================================
Demonstrates HMAC-SHA256 signing, nonce generation,
and making a signed request to /api/v1/address/parse.

Usage:
    pip install requests
    python clients/python/example.py
"""

import hashlib
import hmac
import base64
import time
import json
import requests

# ─── Configuration ────────────────────────────────────────────────────────────
BASE_URL  = "http://localhost:8080"
APP_ID    = "client_001"   # matches APP_IDS in .env
APP_SECRET = "secret1"     # matches APP_SECRETS in .env
# ─────────────────────────────────────────────────────────────────────────────


def generate_nonce() -> str:
    """Generate a unique nonce string (UUID v4)."""
    import uuid
    return str(uuid.uuid4())


def compute_signature(timestamp: str, body: str) -> str:
    """
    HMAC-SHA256 signature as used by the server.
    message = timestamp + body
    signature = base64(HMAC-SHA256(message, app_secret))
    """
    message = timestamp + body
    mac = hmac.new(
        key=APP_SECRET.encode("utf-8"),
        msg=message.encode("utf-8"),
        digestmod=hashlib.sha256,
    )
    return base64.b64encode(mac.digest()).decode("utf-8")


def parse_address(name: str, phone: str, company: str, address: str) -> dict:
    """
    Parse a Chinese address using the /api/v1/address/parse endpoint.
    Returns the parsed JSON response.
    """
    timestamp = str(int(time.time()))
    body = json.dumps(
        {"name": name, "phone": phone, "company": company, "address": address},
        ensure_ascii=False,
    )

    signature = compute_signature(timestamp, body)

    headers = {
        "Content-Type":                "application/json",
        "X-App-Id":                   APP_ID,
        "X-Timestamp":                timestamp,
        "X-Signature":                signature,
        "X-Nonce":                    generate_nonce(),
    }

    resp = requests.post(
        f"{BASE_URL}/api/v1/address/parse",
        headers=headers,
        data=body.encode("utf-8"),
        timeout=30,
    )

    if not resp.ok:
        raise RuntimeError(f"request failed [{resp.status_code}]: {resp.text}")

    return resp.json()


def main():
    # ── Example 1: Full address with all four input fields ──────────────────────
    result = parse_address(
        name="张三",
        phone="15361237638",
        company="智腾达软件技术公司",
        address="广东省深圳市南山区桃源街道大学城创业园桑泰大厦13楼1303室",
    )

    print("=== Example 1: Full Address ===")
    print(json.dumps(result, indent=2, ensure_ascii=False))

    # ── Example 2: Minimal — address only ───────────────────────────────────────
    result2 = parse_address(
        name="",
        phone="",
        company="",
        address="北京市朝阳区建国路88号SOHO现代城A座1001",
    )

    print("\n=== Example 2: Address Only ===")
    print(json.dumps(result2, indent=2, ensure_ascii=False))

    # ── Example 3: Company name with noise (suffix stripping demo) ──────────────
    result3 = parse_address(
        name="李四",
        phone="13900001111",
        company="深圳市智腾达软件技术有限公司",
        address="深圳宝安区西乡街道固戍社区南昌公园旁",
    )

    print("\n=== Example 3: Company + Sub-district ===")
    print(json.dumps(result3, indent=2, ensure_ascii=False))


if __name__ == "__main__":
    main()
