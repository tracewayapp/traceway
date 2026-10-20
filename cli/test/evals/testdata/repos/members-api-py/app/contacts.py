"""Contact search and CSV imports."""

import csv
import io

from fastapi import APIRouter, Depends, HTTPException, UploadFile
from sqlalchemy import desc, func, insert, select

from app.db import contacts, engine
from app.deps import current_account_id
from app.tracing import tracer

router = APIRouter()


def parse_contacts(raw: bytes, account_id: int) -> list[dict]:
    rows = []
    for rec in csv.DictReader(io.StringIO(raw.decode("utf-8-sig"))):
        email = (rec.get("email") or "").strip().lower()
        if "@" not in email:
            continue
        tags = sorted({t.strip() for t in (rec.get("tags") or "").split(";") if t.strip()})
        rows.append({"account_id": account_id, "email": email, "name": (rec.get("name") or "").strip(), "tags": tags})
    return rows


@router.post("/api/imports/contacts")
async def import_contacts(file: UploadFile, account_id: int = Depends(current_account_id)):
    raw = await file.read()
    with tracer.start_as_current_span("parse csv"):
        rows = parse_contacts(raw, account_id)
    if not rows:
        raise HTTPException(status_code=422, detail="no valid contacts in the file")
    with engine.begin() as conn:
        conn.execute(insert(contacts).values(rows))
    return {"imported": len(rows)}


@router.get("/api/search")
def search(q: str, account_id: int = Depends(current_account_id)):
    tsq = func.plainto_tsquery(q)
    stmt = (
        select(contacts.c.id, contacts.c.name, contacts.c.email)
        .where(contacts.c.account_id == account_id, contacts.c.search_vector.op("@@")(tsq))
        .order_by(desc(func.ts_rank(contacts.c.search_vector, tsq)))
        .limit(20)
    )
    with engine.connect() as conn:
        return {"contacts": list(conn.execute(stmt).mappings())}
