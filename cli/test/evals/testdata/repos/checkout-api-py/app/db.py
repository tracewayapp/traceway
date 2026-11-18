import os

from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

engine = create_engine(
    os.environ.get("DATABASE_URL", "postgresql+psycopg://checkout@checkout-db.internal:5432/checkout"),
    pool_size=10,
    max_overflow=5,
    pool_timeout=30,
    pool_pre_ping=True,
)
SessionLocal = sessionmaker(engine, expire_on_commit=False)
