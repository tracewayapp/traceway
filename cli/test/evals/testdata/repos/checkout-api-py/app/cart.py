import uuid

from fastapi import APIRouter, Header
from sqlalchemy import select

from app.db import SessionLocal
from app.models import CartItem

router = APIRouter(prefix="/api")


@router.get("/cart")
def get_cart(x_cart_id: str = Header()):
    cart_id = uuid.UUID(x_cart_id)
    # A short session: the connection goes back to the pool as soon as the
    # rows are read.
    with SessionLocal() as session:
        rows = session.execute(select(CartItem).where(CartItem.cart_id == cart_id)).scalars().all()
    return {
        "items": [{"sku": r.sku, "qty": r.qty, "unitPriceCents": r.unit_price_cents} for r in rows],
        "totalCents": sum(r.qty * r.unit_price_cents for r in rows),
    }
