from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, EmailStr
from sqlalchemy import select, update
from sqlalchemy.ext.asyncio import AsyncSession

from app.db import current_user, get_session
from app.models import Org, User

router = APIRouter(prefix="/api")


class UserOut(BaseModel):
    id: int
    email: str
    display_name: str
    org_id: int
    org_name: str | None = None


@router.get("/me", response_model=UserOut)
async def me(user_id: int = Depends(current_user), session: AsyncSession = Depends(get_session)):
    user = await session.get(User, user_id)
    org = await session.get(Org, user.org_id)
    return UserOut(id=user.id, email=user.email, display_name=user.display_name, org_id=user.org_id, org_name=org.name)


class ProfileIn(BaseModel):
    display_name: str


@router.patch("/me", response_model=UserOut)
async def update_me(body: ProfileIn, user_id: int = Depends(current_user), session: AsyncSession = Depends(get_session)):
    await session.execute(update(User).where(User.id == user_id).values(display_name=body.display_name))
    await session.commit()
    return await me(user_id, session)


@router.get("/users/lookup", response_model=UserOut)
async def lookup(email: EmailStr, _: int = Depends(current_user), session: AsyncSession = Depends(get_session)):
    """Find a user by email address: the invite dialog and the admin user search."""
    user = (await session.execute(select(User).where(User.email == email))).scalar_one_or_none()
    if user is None:
        raise HTTPException(status_code=404, detail="no user with that email")
    org = await session.get(Org, user.org_id)
    return UserOut(id=user.id, email=user.email, display_name=user.display_name, org_id=user.org_id, org_name=org.name)
