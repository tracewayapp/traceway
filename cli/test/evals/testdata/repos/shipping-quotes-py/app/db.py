import os

from psycopg.rows import dict_row
from psycopg_pool import AsyncConnectionPool

pool = AsyncConnectionPool(os.environ.get("DATABASE_URL", ""), kwargs={"row_factory": dict_row}, open=False)
