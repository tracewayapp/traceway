"""Database engine and table definitions (SQLAlchemy Core)."""

import os

from sqlalchemy import BigInteger, Column, DateTime, Integer, MetaData, String, Table, create_engine
from sqlalchemy.dialects.postgresql import ARRAY, TSVECTOR

metadata = MetaData()

accounts = Table(
    "accounts", metadata,
    Column("id", BigInteger, primary_key=True),
    Column("email", String), Column("display_name", String), Column("plan", String),
    Column("password_hash", String), Column("avatar_key", String), Column("created_at", DateTime),
)

orders = Table(
    "orders", metadata,
    Column("id", BigInteger, primary_key=True),
    Column("account_id", BigInteger, index=True),
    Column("status", String), Column("total_cents", BigInteger), Column("created_at", DateTime),
)

order_items = Table(
    "order_items", metadata,
    Column("id", BigInteger, primary_key=True),
    Column("order_id", BigInteger, index=True),
    Column("sku", String), Column("qty", Integer), Column("unit_price_cents", BigInteger),
)

contacts = Table(
    "contacts", metadata,
    Column("id", BigInteger, primary_key=True),
    Column("account_id", BigInteger, index=True),
    Column("email", String), Column("name", String), Column("tags", ARRAY(String)),
    Column("search_vector", TSVECTOR),
)

activity_events = Table(
    "activity_events", metadata,
    Column("id", BigInteger, primary_key=True),
    Column("account_id", BigInteger, index=True),
    Column("kind", String), Column("created_at", DateTime),
)

sessions = Table(
    "sessions", metadata,
    Column("id", BigInteger, primary_key=True),
    Column("account_id", BigInteger),
    Column("token_hash", String), Column("expires_at", DateTime),
)

engine = create_engine(os.environ.get("DATABASE_URL", "postgresql+psycopg2://members@localhost/members"), pool_size=10)
