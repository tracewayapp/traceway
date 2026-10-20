"""PostgreSQL persistence for payments and subscriptions."""

from dataclasses import dataclass


@dataclass
class Account:
    id: int
    customer_id: str


class Store:
    def __init__(self, conn):
        self.conn = conn

    def account_for_customer(self, customer_id: str) -> Account | None:
        row = self.conn.execute(
            "SELECT id, customer_id FROM accounts WHERE customer_id = %s", (customer_id,)
        ).fetchone()
        return Account(*row) if row else None

    def record_payment(self, invoice_id: str, amount_cents: int, currency: str, account_id: int | None) -> None:
        self.conn.execute(
            "INSERT INTO payments (invoice_id, amount_cents, currency, account_id, paid_at) "
            "VALUES (%s, %s, %s, %s, now()) ON CONFLICT (invoice_id) DO NOTHING",
            (invoice_id, amount_cents, currency, account_id),
        )

    def activate_subscription(self, account_id: int, subscription_id: str) -> None:
        self.conn.execute(
            "UPDATE subscriptions SET status = 'active', updated_at = now() WHERE account_id = %s AND provider_id = %s",
            (account_id, subscription_id),
        )

    def cancel_subscription(self, subscription_id: str) -> None:
        self.conn.execute(
            "UPDATE subscriptions SET status = 'cancelled', updated_at = now() WHERE provider_id = %s",
            (subscription_id,),
        )

    def payment(self, invoice_id: str) -> dict | None:
        row = self.conn.execute(
            "SELECT invoice_id, amount_cents, currency, account_id, paid_at FROM payments WHERE invoice_id = %s", (invoice_id,)
        ).fetchone()
        if row is None:
            return None
        return dict(zip(("invoice_id", "amount_cents", "currency", "account_id", "paid_at"), row))
