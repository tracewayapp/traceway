import tempfile
import unittest
from pathlib import Path

from app.config import load_settings

GOOD = b"""
[database]
pool_size = 4
pool_timeout_seconds = 2

[recommendations]
enabled = true
base_url = "http://localhost:9000"
model = "m"
timeout_ms = 500
max_items = 6
"""


class LoadSettingsTest(unittest.TestCase):
    def load(self, body: bytes):
        with tempfile.NamedTemporaryFile(suffix=".toml", delete=False) as f:
            f.write(body)
        return load_settings(Path(f.name))

    def test_valid_settings(self):
        s = self.load(GOOD)
        self.assertEqual(s.pool_size, 4)
        self.assertEqual(s.recommendations.timeout_ms, 500)

    def test_bad_recommendations_block_disables_the_feature(self):
        with self.assertLogs("app.config", level="ERROR"):
            s = self.load(GOOD.replace(b"max_items = 6", b'max_items = "six"'))
        self.assertIsNone(s.recommendations)


if __name__ == "__main__":
    unittest.main()
