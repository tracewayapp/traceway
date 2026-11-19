"""HTTP entry point."""

import logging
import os

from fastapi import FastAPI
from fastapi.responses import JSONResponse
from sqlalchemy import create_engine, text

from app.access_log import AccessLog
from app.config import load_settings

log = logging.getLogger("app.main")
settings = load_settings()
engine = create_engine(os.environ["DATABASE_URL"], pool_size=settings.pool_size, pool_timeout=settings.pool_timeout_seconds, hide_parameters=True)
app = FastAPI()
app.add_middleware(AccessLog)

recs = None
if settings.recommendations is not None:
    from app.recommendations import RecommendationsClient

    recs = RecommendationsClient(settings.recommendations)


@app.get("/api/products/{product_id}")
def product(product_id: int):
    with engine.connect() as conn:
        row = conn.execute(text("SELECT id, name, price_cents, currency FROM products WHERE id = :id"), {"id": product_id}).mappings().first()
    if row is None:
        return JSONResponse({"error": "not found"}, status_code=404)
    return dict(row)


@app.get("/api/products/{product_id}/recommendations")
async def recommendations(product_id: int):
    if recs is None:
        log.error("recommendations requested but no recommendations client is configured")
        return JSONResponse({"error": "recommendations unavailable"}, status_code=500)
    return {"items": await recs.similar(product_id)}
