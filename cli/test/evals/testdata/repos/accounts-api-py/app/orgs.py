from fastapi import APIRouter, Depends
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.db import current_user, get_session
from app.models import OrgMember, User

router = APIRouter(prefix="/api")


@router.get("/orgs/{org_id}/members")
async def members(org_id: int, limit: int = 50, _: int = Depends(current_user), session: AsyncSession = Depends(get_session)):
    rows = await session.execute(
        select(User.id, User.email, User.display_name, OrgMember.role)
        .select_from(OrgMember)
        .join(User, User.id == OrgMember.user_id)
        .where(OrgMember.org_id == org_id)
        .order_by(User.display_name)
        .limit(min(limit, 200))
    )
    return [dict(r._mapping) for r in rows]
