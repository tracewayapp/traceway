"""Activity report and recommendations."""

import datetime
import os

import httpx
from fastapi import APIRouter, Depends
from sqlalchemy import func, select

from app.db import activity_events, engine
from app.deps import current_account_id

router = APIRouter()

RECS_URL = os.environ.get("RECS_URL", "http://recs-model.internal:8501/v1/models/members:predict")
recs_client = httpx.Client(timeout=3.0)


@router.get("/api/reports/activity")
def activity(days: int = 90, account_id: int = Depends(current_account_id)):
    since = datetime.datetime.now(datetime.UTC) - datetime.timedelta(days=days)
    stmt = (
        select(activity_events.c.kind, func.count())
        .where(activity_events.c.account_id == account_id, activity_events.c.created_at >= since)
        .group_by(activity_events.c.kind)
    )
    with engine.connect() as conn:
        return {"since": since.isoformat(), "byKind": {kind: n for kind, n in conn.execute(stmt)}}


@router.get("/api/recommendations")
def recommendations(account_id: int = Depends(current_account_id)):
    resp = recs_client.post(RECS_URL, json={"instances": [{"account_id": account_id}]})
    resp.raise_for_status()
    return {"items": resp.json()["predictions"][0]}
