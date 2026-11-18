import unittest

from app.carriers.rates import Rate, cheapest, parse_rates


class RatesTest(unittest.TestCase):
    def test_parse_and_pick_cheapest(self):
        body = {
            "rates": [
                {"carrier": "dhl", "service": "express", "amount": {"minor": 1490, "currency": "EUR"}, "transit_days": 1},
                {"carrier": "dpd", "service": "classic", "amount": {"minor": 690, "currency": "EUR"}, "transit_days": 3},
                {"carrier": "gls", "service": "business", "amount": {"minor": 690, "currency": "EUR"}, "transit_days": 2},
            ]
        }
        rates = parse_rates(body)
        self.assertEqual(len(rates), 3)
        self.assertEqual(cheapest(rates), Rate("gls", "business", 690, "EUR", 2))

    def test_no_rates(self):
        self.assertIsNone(cheapest(parse_rates({})))


if __name__ == "__main__":
    unittest.main()
