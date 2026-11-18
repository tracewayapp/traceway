from fastapi import APIRouter

from app.db import pool

router = APIRouter()


@router.get("/api/shipments/{shipment_id}/tracking")
async def tracking(shipment_id: int):
    async with pool.connection() as conn:
        rows = await (await conn.execute(
            "SELECT status, location, occurred_at FROM tracking_events WHERE shipment_id = %s ORDER BY occurred_at DESC LIMIT 20",
            (shipment_id,),
        )).fetchall()
    return {"events": rows}
