import os

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from app.carriers.client import CarrierClient
from app.carriers.rates import cheapest
from app.db import pool

router = APIRouter()
carrier = CarrierClient(os.environ.get("CARRIER_API_KEY", ""))


class QuoteRequest(BaseModel):
    parcel_id: int


@router.post("/api/shipping/quotes")
async def create_quote(body: QuoteRequest):
    async with pool.connection() as conn:
        parcel = await (await conn.execute(
            "SELECT id, weight_grams, length_mm, width_mm, height_mm, origin_postcode, destination_postcode, destination_country FROM parcels WHERE id = %s",
            (body.parcel_id,),
        )).fetchone()
    if parcel is None:
        raise HTTPException(status_code=404, detail="parcel not found")
    rates = await carrier.fetch_rates(parcel)
    best = cheapest(rates)
    if best is None:
        raise HTTPException(status_code=422, detail="no carrier service for this parcel")
    async with pool.connection() as conn:
        await conn.execute(
            "INSERT INTO quotes (parcel_id, carrier, service_level, amount_minor, currency, expires_at) VALUES (%s, %s, %s, %s, %s, %s)",
            (parcel["id"], best.carrier, best.service_level, best.amount_minor, best.currency, None),
        )
    return {"carrier": best.carrier, "service": best.service_level, "amount_minor": best.amount_minor, "currency": best.currency, "rates": rates}
