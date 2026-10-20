from fastapi import FastAPI, HTTPException
from opentelemetry import trace
from pydantic import BaseModel, Field

from .db import pool
from .pricing.models import Price, Promotion
from .pricing.quote import UnknownSku, build_quote

app = FastAPI(title="pricing-service")


class QuoteItem(BaseModel):
    sku: str
    quantity: int = Field(gt=0)


class QuoteRequest(BaseModel):
    items: list[QuoteItem] = Field(min_length=1)


@app.post("/api/quotes")
async def create_quote(req: QuoteRequest):
    trace.get_current_span().set_attribute("cart.items", len(req.items))
    skus = [i.sku for i in req.items]
    async with pool.connection() as conn:
        rows = await (await conn.execute(
            "SELECT sku, family, unit_price_cents, tax_class FROM prices WHERE sku = ANY(%(skus)s)", {"skus": skus})).fetchall()
        promos = await (await conn.execute(
            "SELECT id, kind, family, sku, partner_sku, min_quantity, percent, amount_cents FROM promotions"
            " WHERE active AND starts_at <= now() AND ends_at > now()")).fetchall()
    prices = {r[0]: Price(*r) for r in rows}
    promotions = [Promotion(*p) for p in promos]
    try:
        quote = build_quote([i.model_dump() for i in req.items], prices, promotions)
    except UnknownSku as e:
        raise HTTPException(status_code=422, detail=f"unknown sku {e}")
    return {
        "lines": [l.__dict__ for l in quote.lines],
        "subtotalCents": quote.subtotal_cents,
        "discountCents": quote.discount_cents,
        "taxCents": quote.tax_cents,
        "totalCents": quote.total_cents,
    }


@app.get("/api/price-lists/{price_list_id}")
async def price_list(price_list_id: int):
    async with pool.connection() as conn:
        rows = await (await conn.execute(
            "SELECT sku, family, unit_price_cents, tax_class FROM prices WHERE price_list_id = %(price_list_id)s ORDER BY sku LIMIT 200",
            {"price_list_id": price_list_id})).fetchall()
    return [Price(*r).__dict__ for r in rows]
