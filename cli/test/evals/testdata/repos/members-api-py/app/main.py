"""HTTP routes."""

import logging

from fastapi import Depends, FastAPI, HTTPException

from app import access_log, auth, contacts, insights, media, queries
from app.db import engine
from app.deps import current_account_id
from app.summary import build_summary

logging.basicConfig(level=logging.INFO)
logging.getLogger("httpx").setLevel(logging.WARNING)

app = FastAPI(title="members-api")
access_log.install(app)
for module in (auth, contacts, insights, media):
    app.include_router(module.router)


@app.get("/api/account/summary")
def account_summary(account_id: int = Depends(current_account_id)):
    with engine.connect() as conn:
        acc = queries.account(conn, account_id)
        if acc is None:
            raise HTTPException(status_code=404, detail="account not found")
        return build_summary(
            acc,
            queries.order_count(conn, account_id),
            queries.recent_orders(conn, account_id),
            queries.monthly_spend(conn, account_id),
        )
