import os

import httpx

_client = httpx.Client(
    base_url="https://api.payments.example",
    headers={"Authorization": f"Bearer {os.environ.get('PAYMENTS_API_KEY', '')}"},
    timeout=httpx.Timeout(10.0, connect=3.0),
)


class PaymentDeclined(Exception):
    pass


def charge(user_id: int, amount_cents: int, idempotency_key: str) -> str:
    """Charges the user's saved card and returns the charge id."""
    resp = _client.post(
        "/v1/charges",
        json={"customer": f"user_{user_id}", "amount": amount_cents, "currency": "eur"},
        headers={"Idempotency-Key": idempotency_key},
    )
    if resp.status_code == 402:
        raise PaymentDeclined(resp.json().get("message", "card declined"))
    resp.raise_for_status()
    return resp.json()["id"]
