"""Read queries of the account page."""

from sqlalchemy import Connection, desc, func, select

from app.db import accounts, order_items, orders


def account(conn: Connection, account_id: int):
    q = select(accounts.c.id, accounts.c.email, accounts.c.display_name, accounts.c.plan, accounts.c.created_at).where(
        accounts.c.id == account_id
    )
    return conn.execute(q).mappings().first()


def order_count(conn: Connection, account_id: int) -> int:
    return conn.execute(select(func.count()).select_from(orders).where(orders.c.account_id == account_id)).scalar_one()


def recent_orders(conn: Connection, account_id: int, limit: int = 5):
    q = (
        select(orders.c.id, orders.c.status, orders.c.total_cents, orders.c.created_at)
        .where(orders.c.account_id == account_id)
        .order_by(desc(orders.c.created_at))
        .limit(limit)
    )
    return list(conn.execute(q).mappings())


def monthly_spend(conn: Connection, account_id: int):
    """Orders and spend per calendar month over the account's whole history."""
    month = func.date_trunc("month", orders.c.created_at)
    q = (
        select(
            month.label("month"),
            func.count(orders.c.id.distinct()).label("orders"),
            func.sum(order_items.c.qty * order_items.c.unit_price_cents).label("spend_cents"),
        )
        .select_from(orders.join(order_items, order_items.c.order_id == orders.c.id))
        .where(orders.c.account_id == account_id)
        .group_by(month)
        .order_by("month")
    )
    return list(conn.execute(q).mappings())
