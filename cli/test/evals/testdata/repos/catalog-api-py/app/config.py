"""Loads and validates config/settings.toml at startup."""

import logging
import tomllib
from dataclasses import dataclass
from pathlib import Path

log = logging.getLogger("app.config")

DEFAULT_PATH = Path(__file__).resolve().parent.parent / "config" / "settings.toml"


class ConfigError(ValueError):
    pass


@dataclass
class RecommendationsSettings:
    enabled: bool
    base_url: str
    model: str
    timeout_ms: int
    max_items: int


@dataclass
class Settings:
    pool_size: int
    pool_timeout_seconds: int
    recommendations: RecommendationsSettings | None
    product_ttl_seconds: int


def _int(section: dict, key: str, where: str) -> int:
    value = section[key]
    if not isinstance(value, int) or isinstance(value, bool):
        raise ConfigError(f"{where}.{key} must be an integer, got {value!r}")
    return value


def _recommendations(raw: dict) -> RecommendationsSettings | None:
    section = raw.get("recommendations", {})
    if not section.get("enabled", False):
        return None
    try:
        return RecommendationsSettings(
            enabled=True,
            base_url=section["base_url"],
            model=section["model"],
            timeout_ms=_int(section, "timeout_ms", "recommendations"),
            max_items=_int(section, "max_items", "recommendations"),
        )
    except (KeyError, ConfigError) as exc:
        # A bad recommendations block must not keep the product pages down.
        log.error("invalid config: %s; recommendations are disabled", exc)
        return None


def load_settings(path: Path = DEFAULT_PATH) -> Settings:
    with open(path, "rb") as f:
        raw = tomllib.load(f)
    db = raw["database"]
    cache = raw.get("cache", {})
    return Settings(
        pool_size=_int(db, "pool_size", "database"),
        pool_timeout_seconds=_int(db, "pool_timeout_seconds", "database"),
        recommendations=_recommendations(raw),
        product_ttl_seconds=_int(cache, "product_ttl_seconds", "cache") if "product_ttl_seconds" in cache else 300,
    )
