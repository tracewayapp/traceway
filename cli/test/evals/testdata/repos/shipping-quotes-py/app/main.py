import logging
from contextlib import asynccontextmanager

from fastapi import FastAPI

from app import access_log
from app.db import pool
from app.routes import quotes, tracking

logging.basicConfig(level=logging.INFO)
logging.getLogger("httpx").setLevel(logging.WARNING)


@asynccontextmanager
async def lifespan(_: FastAPI):
    await pool.open()
    yield
    await quotes.carrier.aclose()
    await pool.close()


app = FastAPI(title="quotes-api", lifespan=lifespan)
access_log.install(app)
app.include_router(quotes.router)
app.include_router(tracking.router)
