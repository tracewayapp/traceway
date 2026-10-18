import logging
import uuid

from fastapi import APIRouter, Header, HTTPException
from pydantic import BaseModel
from sqlalchemy import delete, select, update
from sqlalchemy.exc import TimeoutError as PoolTimeoutError

from app import payments
from app.db import SessionLocal
from app.models import CartItem, Inventory, Order

log = logging.getLogger(__name__)
router = APIRouter(prefix="/api")


class CheckoutIn(BaseModel):
    cart_id: uuid.UUID


class CheckoutOut(BaseModel):
    order_id: uuid.UUID
    total_cents: int


@router.post("/checkout", status_code=201, response_model=CheckoutOut)
def create_checkout(body: CheckoutIn, x_user_id: int = Header()):
    """Places the order for a cart: checks stock, charges the card, writes the
    order and decrements inventory, all in one transaction so a failed charge
    leaves nothing behind."""
    try:
        return _checkout(body, x_user_id)
    except PoolTimeoutError as exc:
        log.warning("checkout failed for cart %s: %s", body.cart_id, exc)
        raise


def _checkout(body: CheckoutIn, user_id: int) -> CheckoutOut:
    with SessionLocal.begin() as session:
        items = session.execute(select(CartItem).where(CartItem.cart_id == body.cart_id)).scalars().all()
        if not items:
            raise HTTPException(status_code=404, detail="cart is empty")
        skus = [i.sku for i in items]
        stock = dict(session.execute(select(Inventory.sku, Inventory.available).where(Inventory.sku.in_(skus))).all())
        for item in items:
            if stock.get(item.sku, 0) < item.qty:
                raise HTTPException(status_code=409, detail=f"{item.sku} is out of stock")

        total = sum(i.qty * i.unit_price_cents for i in items)
        payments.charge(user_id, total, idempotency_key=str(body.cart_id))

        order = Order(cart_id=body.cart_id, user_id=user_id, total_cents=total, status="paid")
        session.add(order)
        session.flush()
        for item in items:
            session.execute(
                update(Inventory).where(Inventory.sku == item.sku).values(available=Inventory.available - item.qty)
            )
        session.execute(delete(CartItem).where(CartItem.cart_id == body.cart_id))
    return CheckoutOut(order_id=order.id, total_cents=total)
