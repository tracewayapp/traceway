"""Applies the payment provider's webhook events.

Events look like {"id": "evt_...", "type": "invoice.paid", "data": {"object": {...}}}.
The invoice object carries id, amount_paid (cents), currency and, when the
payer has a customer record at the provider, customer and subscription.
"""

import hashlib
import hmac
import time


class SignatureError(Exception):
    """The request was not signed with our webhook secret."""


def verify_signature(payload: bytes, header: str, secret: str, tolerance: int = 300) -> None:
    """Checks a "t=<unix>,v1=<hex>" signature header."""
    parts = dict(p.split("=", 1) for p in header.split(",") if "=" in p)
    try:
        ts = int(parts["t"])
        sig = parts["v1"]
    except (KeyError, ValueError) as exc:
        raise SignatureError("malformed signature header") from exc
    if abs(time.time() - ts) > tolerance:
        raise SignatureError("signature timestamp outside tolerance")
    expected = hmac.new(secret.encode(), f"{ts}.".encode() + payload, hashlib.sha256).hexdigest()
    if not hmac.compare_digest(expected, sig):
        raise SignatureError("signature mismatch")


def handle_event(event: dict, store) -> dict:
    """Dispatches an event to its handler; unknown types are acknowledged."""
    handler = HANDLERS.get(event["type"])
    if handler is None:
        return {"ignored": event["type"]}
    return handler(event["data"]["object"], store)


def _invoice_paid(invoice: dict, store) -> dict:
    customer_id = invoice["customer"]
    account = store.account_for_customer(customer_id)
    store.record_payment(
        invoice_id=invoice["id"],
        amount_cents=invoice["amount_paid"],
        currency=invoice["currency"],
        account_id=account.id if account else None,
    )
    if account and invoice.get("subscription"):
        store.activate_subscription(account.id, invoice["subscription"])
    return {"recorded": invoice["id"]}


def _subscription_deleted(subscription: dict, store) -> dict:
    store.cancel_subscription(subscription["id"])
    return {"cancelled": subscription["id"]}


HANDLERS = {
    "invoice.paid": _invoice_paid,
    "customer.subscription.deleted": _subscription_deleted,
}
