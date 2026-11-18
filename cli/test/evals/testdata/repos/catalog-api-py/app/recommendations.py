"""Client of the recommendations service."""

import httpx

from app.config import RecommendationsSettings


class RecommendationsClient:
    def __init__(self, settings: RecommendationsSettings):
        self.settings = settings
        self.http = httpx.AsyncClient(base_url=settings.base_url, timeout=settings.timeout_ms / 1000)

    async def similar(self, product_id: int) -> list[dict]:
        resp = await self.http.get(
            "/v1/similar",
            params={"product_id": product_id, "model": self.settings.model, "limit": self.settings.max_items},
        )
        resp.raise_for_status()
        return resp.json()["items"]
