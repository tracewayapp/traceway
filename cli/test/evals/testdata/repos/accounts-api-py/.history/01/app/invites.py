import secrets
from datetime import datetime, timezone

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, EmailStr
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.db import current_user, get_session
from app.models import Invite, OrgMember

router = APIRouter(prefix="/api")


class InviteIn(BaseModel):
    org_id: int
    email: EmailStr
    role: str = "member"


@router.get("/invites")
async def list_invites(org_id: int, _: int = Depends(current_user), session: AsyncSession = Depends(get_session)):
    rows = await session.execute(
        select(Invite.id, Invite.email, Invite.role, Invite.created_at)
        .where(Invite.org_id == org_id, Invite.accepted_at.is_(None))
        .order_by(Invite.created_at.desc())
    )
    return [dict(r._mapping) for r in rows]


@router.post("/invites", status_code=201)
async def create_invite(body: InviteIn, user_id: int = Depends(current_user), session: AsyncSession = Depends(get_session)):
    role = (
        await session.execute(select(OrgMember.role).where(OrgMember.org_id == body.org_id, OrgMember.user_id == user_id))
    ).scalar_one_or_none()
    if role not in ("owner", "admin"):
        raise HTTPException(status_code=403, detail="only admins can invite")
    invite = Invite(
        org_id=body.org_id,
        email=body.email,
        role=body.role,
        token=secrets.token_urlsafe(32),
        invited_by=user_id,
        created_at=datetime.now(timezone.utc).replace(tzinfo=None),
    )
    session.add(invite)
    await session.commit()
    return {"id": invite.id, "token": invite.token}


@router.post("/invites/{token}/accept")
async def accept_invite(token: str, user_id: int = Depends(current_user), session: AsyncSession = Depends(get_session)):
    invite = (await session.execute(select(Invite).where(Invite.token == token))).scalar_one_or_none()
    if invite is None or invite.accepted_at is not None:
        raise HTTPException(status_code=404, detail="invite not found")
    session.add(OrgMember(org_id=invite.org_id, user_id=user_id, role=invite.role))
    invite.accepted_at = datetime.now(timezone.utc).replace(tzinfo=None)
    await session.commit()
    return {"orgId": invite.org_id}
