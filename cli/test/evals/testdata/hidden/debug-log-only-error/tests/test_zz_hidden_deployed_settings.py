import logging
import unittest

from app.config import DEFAULT_PATH, load_settings


class HiddenDeployedSettingsTest(unittest.TestCase):
    def test_deployed_settings_enable_recommendations(self):
        logger = logging.getLogger("app.config")
        records = []

        class Capture(logging.Handler):
            def emit(self, record):
                records.append(record)

        handler = Capture(level=logging.ERROR)
        logger.addHandler(handler)
        try:
            settings = load_settings(DEFAULT_PATH)
        finally:
            logger.removeHandler(handler)
        self.assertEqual([r.getMessage() for r in records], [], "loading the deployed settings logs errors")
        recs = settings.recommendations
        self.assertIsNotNone(recs, "recommendations are disabled by the deployed settings")
        self.assertIsInstance(recs.timeout_ms, int)
        self.assertGreater(recs.timeout_ms, 0)
        self.assertLessEqual(recs.timeout_ms, 5000)
        self.assertEqual(recs.model, "similar-items-v3")


if __name__ == "__main__":
    unittest.main()
