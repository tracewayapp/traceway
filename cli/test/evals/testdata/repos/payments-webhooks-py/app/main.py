"""HTTP entry point: the provider posts every event to /api/webhooks/payments."""

import json
import logging
import os

import psycopg
from fastapi import FastAPI, HTTPException, Request
from opentelemetry import trace

from app.access_log import AccessLog
from app.store import Store
from app.webhooks import SignatureError, handle_event, verify_signature

log = logging.getLogger("app.webhooks")
app = FastAPI()
app.add_middleware(AccessLog)
conn = psycopg.connect(os.environ["DATABASE_URL"], autocommit=True)
store = Store(conn)
SECRET = os.environ["WEBHOOK_SECRET"]


@app.post("/api/webhooks/payments")
async def payments_webhook(request: Request):
    payload = await request.body()
    try:
        verify_signature(payload, request.headers.get("x-signature", ""), SECRET)
    except SignatureError as exc:
        log.warning("webhook signature verification failed: %s", exc)
        raise HTTPException(status_code=401, detail="invalid signature")
    event = json.loads(payload)
    span = trace.get_current_span()
    span.set_attribute("payment.event.id", event["id"])
    span.set_attribute("payment.event.type", event["type"])

    result = handle_event(event, store)
    return {"ok": True, **result}


@app.get("/api/payments/{invoice_id}")
def payment(invoice_id: str):
    found = store.payment(invoice_id)
    if found is None:
        raise HTTPException(status_code=404, detail="payment not found")
    return found
