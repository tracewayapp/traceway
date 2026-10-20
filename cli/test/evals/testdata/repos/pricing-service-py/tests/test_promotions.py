import unittest

from app.pricing.models import Line, Promotion
from app.pricing.promotions import apply_promotions


def line(sku, family, qty, price=1000):
    return Line(sku=sku, family=family, quantity=qty, unit_price_cents=price, tax_class="standard")


class ApplyPromotionsTest(unittest.TestCase):
    def test_volume_discount_needs_enough_units_of_the_family(self):
        promos = [Promotion(id=1, kind="volume", family="paper", min_quantity=10, percent=15)]
        few = apply_promotions([line("A4-500", "paper", 4), line("A3-250", "paper", 3)], promos)
        self.assertEqual([l.discount_cents for l in few], [0, 0])
        many = apply_promotions([line("A4-500", "paper", 6), line("A3-250", "paper", 5)], promos)
        self.assertEqual([l.discount_cents for l in many], [900, 750])

    def test_bundle_needs_the_partner_sku(self):
        promos = [Promotion(id=2, kind="bundle", sku="TONER-K", partner_sku="PRINTER-X1", amount_cents=500)]
        alone = apply_promotions([line("TONER-K", "toner", 2)], promos)
        self.assertEqual(alone[0].discount_cents, 0)
        both = apply_promotions([line("TONER-K", "toner", 2), line("PRINTER-X1", "printers", 1)], promos)
        self.assertEqual(both[0].discount_cents, 1000)
        self.assertEqual(both[0].applied, [2])

    def test_best_promotion_wins(self):
        promos = [
            Promotion(id=1, kind="volume", family="toner", min_quantity=2, percent=10),
            Promotion(id=2, kind="bundle", sku="TONER-K", partner_sku="PRINTER-X1", amount_cents=500),
        ]
        out = apply_promotions([line("TONER-K", "toner", 2), line("PRINTER-X1", "printers", 1)], promos)
        self.assertEqual(out[0].discount_cents, 1000)
        self.assertEqual(out[0].applied, [2])


if __name__ == "__main__":
    unittest.main()
