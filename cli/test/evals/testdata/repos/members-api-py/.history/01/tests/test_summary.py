import datetime
import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from app.summary import build_summary  # noqa: E402


class BuildSummaryTest(unittest.TestCase):
    def test_shapes_the_payload(self):
        acc = {"id": 7, "email": "a@example.com", "display_name": "Ada", "plan": "plus",
               "created_at": datetime.datetime(2024, 3, 1, 9, 30)}
        recent = [{"id": 91, "status": "shipped", "total_cents": 4599, "created_at": datetime.datetime(2026, 10, 1, 12, 0)}]
        out = build_summary(acc, 42, recent)
        self.assertEqual(out["account"]["memberSince"], "2024-03-01")
        self.assertEqual(out["orderCount"], 42)
        self.assertEqual(out["recentOrders"][0]["totalCents"], 4599)


if __name__ == "__main__":
    unittest.main()
