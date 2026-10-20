"""HTTP routes."""

from fastapi import Depends, FastAPI, Header, HTTPException

from app import queries
from app.db import engine
from app.summary import build_summary

app = FastAPI(title="members-api")


def current_account_id(x_account_id: int = Header()) -> int:
    return x_account_id


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
        )
