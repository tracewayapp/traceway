import unittest

from fastapi import HTTPException

import main


class InventoryTest(unittest.TestCase):
    def test_get_item(self):
        self.assertEqual(main.get_item(2).sku, "TEE-03")

    def test_unknown_item_is_404(self):
        with self.assertRaises(HTTPException) as ctx:
            main.get_item(99)
        self.assertEqual(ctx.exception.status_code, 404)

    def test_in_stock_filter(self):
        self.assertEqual([i.id for i in main.list_items(in_stock=True)], [1, 2])


if __name__ == "__main__":
    unittest.main()
