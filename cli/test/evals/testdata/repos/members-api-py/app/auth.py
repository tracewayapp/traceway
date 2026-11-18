"""Password login."""

import datetime
import hashlib
import secrets

import bcrypt
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from sqlalchemy import insert, select

from app.db import accounts, engine, sessions
from app.tracing import tracer

router = APIRouter()

SESSION_TTL = datetime.timedelta(days=30)


class Login(BaseModel):
    email: str
    password: str


@router.post("/api/auth/login")
def login(body: Login):
    with engine.begin() as conn:
        acc = conn.execute(
            select(accounts.c.id, accounts.c.password_hash).where(accounts.c.email == body.email.lower())
        ).mappings().first()
        with tracer.start_as_current_span("bcrypt.checkpw"):
            ok = acc is not None and bcrypt.checkpw(body.password.encode(), acc["password_hash"].encode())
        if not ok:
            raise HTTPException(status_code=401, detail="wrong email or password")
        token = secrets.token_urlsafe(32)
        conn.execute(insert(sessions).values(
            account_id=acc["id"],
            token_hash=hashlib.sha256(token.encode()).hexdigest(),
            expires_at=datetime.datetime.now(datetime.UTC) + SESSION_TTL,
        ))
    return {"token": token}
