import unittest

from app.webhooks import handle_event


class FakeStore:
    def __init__(self, accounts=None):
        self.accounts = accounts or {}
        self.payments = []
        self.active = []
        self.cancelled = []

    def account_for_customer(self, customer_id):
        return self.accounts.get(customer_id)

    def record_payment(self, **kw):
        self.payments.append(kw)

    def activate_subscription(self, account_id, subscription_id):
        self.active.append((account_id, subscription_id))

    def cancel_subscription(self, subscription_id):
        self.cancelled.append(subscription_id)


def invoice_paid(**invoice):
    return {"id": "evt_1", "type": "invoice.paid", "data": {"object": invoice}}


class InvoicePaidTest(unittest.TestCase):
    def test_records_payment(self):
        store = FakeStore()
        res = handle_event(invoice_paid(id="in_1", amount_paid=4900, currency="usd", customer="cus_9"), store)
        self.assertEqual(res, {"recorded": "in_1"})
        self.assertEqual(store.payments, [{"invoice_id": "in_1", "amount_cents": 4900, "currency": "usd", "account_id": None}])

    def test_unknown_event_is_acknowledged(self):
        self.assertEqual(handle_event({"id": "evt_2", "type": "charge.dispute.created", "data": {"object": {}}}, FakeStore()), {"ignored": "charge.dispute.created"})


if __name__ == "__main__":
    unittest.main()
