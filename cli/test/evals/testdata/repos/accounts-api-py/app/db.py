import hmac
import os

from fastapi import Cookie, HTTPException
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

engine = create_async_engine(
    os.environ.get("DATABASE_URL", "postgresql+asyncpg://accounts@localhost/accounts"),
    pool_size=10,
    max_overflow=5,
    pool_pre_ping=True,
)
SessionLocal = async_sessionmaker(engine, expire_on_commit=False)
_SESSION_SECRET = os.environ.get("SESSION_SECRET", "dev").encode()


async def get_session() -> AsyncSession:
    async with SessionLocal() as session:
        yield session


async def current_user(session_id: str | None = Cookie(default=None)) -> int:
    """Session cookies are '<user id>.<hmac>'."""
    if not session_id or "." not in session_id:
        raise HTTPException(status_code=401, detail="not signed in")
    user_id, sig = session_id.split(".", 1)
    expected = hmac.new(_SESSION_SECRET, user_id.encode(), "sha256").hexdigest()
    if not hmac.compare_digest(sig, expected):
        raise HTTPException(status_code=401, detail="not signed in")
    return int(user_id)
