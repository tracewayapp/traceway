import unittest

from app.store import Account
from app.webhooks import handle_event
from tests.test_webhooks import FakeStore, invoice_paid


class HiddenGuestInvoiceTest(unittest.TestCase):
    def test_invoice_without_customer_is_recorded(self):
        store = FakeStore({"cus_9": Account(id=12, customer_id="cus_9")})
        res = handle_event(invoice_paid(id="in_guest", amount_paid=2500, currency="eur"), store)
        self.assertEqual(res, {"recorded": "in_guest"})
        self.assertEqual(len(store.payments), 1, "a guest payment must still be recorded")
        p = store.payments[0]
        self.assertEqual((p["invoice_id"], p["amount_cents"], p["currency"]), ("in_guest", 2500, "eur"))
        self.assertIsNone(p["account_id"])
        self.assertEqual(store.active, [])

    def test_customer_with_null_value(self):
        store = FakeStore()
        res = handle_event(invoice_paid(id="in_n", amount_paid=100, currency="usd", customer=None), store)
        self.assertEqual(res, {"recorded": "in_n"})
        self.assertIsNone(store.payments[0]["account_id"])

    def test_customer_invoice_unchanged(self):
        store = FakeStore({"cus_9": Account(id=12, customer_id="cus_9")})
        handle_event(invoice_paid(id="in_2", amount_paid=4900, currency="usd", customer="cus_9", subscription="sub_3"), store)
        self.assertEqual(store.payments[0]["account_id"], 12)
        self.assertEqual(store.active, [(12, "sub_3")])


if __name__ == "__main__":
    unittest.main()
