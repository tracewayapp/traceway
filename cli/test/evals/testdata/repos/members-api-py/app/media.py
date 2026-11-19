"""Avatar uploads."""

import io
import os
import uuid

from fastapi import APIRouter, Depends, UploadFile
from PIL import Image
from sqlalchemy import update

from app.db import accounts, engine
from app.deps import current_account_id
from app.tracing import tracer

router = APIRouter()

MEDIA_ROOT = os.environ.get("MEDIA_ROOT", "/var/lib/members/media")
AVATAR_SIZE = (256, 256)


@router.post("/api/media/avatar")
async def upload_avatar(file: UploadFile, account_id: int = Depends(current_account_id)):
    raw = await file.read()
    with tracer.start_as_current_span("resize image"):
        img = Image.open(io.BytesIO(raw))
        img.thumbnail(AVATAR_SIZE)
        out = io.BytesIO()
        img.convert("RGB").save(out, format="JPEG", quality=85)
    key = f"avatars/{account_id}/{uuid.uuid4().hex}.jpg"
    path = os.path.join(MEDIA_ROOT, key)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "wb") as f:
        f.write(out.getvalue())
    with engine.begin() as conn:
        conn.execute(update(accounts).where(accounts.c.id == account_id).values(avatar_key=key))
    return {"avatarKey": key}
