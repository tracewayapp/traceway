import os

from psycopg_pool import AsyncConnectionPool

pool = AsyncConnectionPool(os.environ.get("DATABASE_URL", "postgresql://pricing@localhost/pricing"), min_size=2, max_size=10, open=False)
