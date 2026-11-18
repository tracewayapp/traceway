import logging

import httpx
from tenacity import (
    before_sleep_log,
    retry,
    retry_if_exception_type,
    stop_after_attempt,
    wait_exponential,
)

from app.carriers.rates import Rate, parse_rates

log = logging.getLogger("app.carriers")

RATES_URL = "https://rates.carrier.example/v2/rates"


class CarrierClient:
    def __init__(self, api_key: str):
        self._http = httpx.AsyncClient(headers={"Authorization": f"Bearer {api_key}"})

    @retry(
        retry=retry_if_exception_type((httpx.TransportError, httpx.HTTPStatusError)),
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=1, max=4),
        before_sleep=before_sleep_log(log, logging.WARNING),
    )
    async def fetch_rates(self, parcel: dict) -> list[Rate]:
        payload = {
            "origin": parcel["origin_postcode"],
            "destination": {"postcode": parcel["destination_postcode"], "country": parcel["destination_country"]},
            "weight_grams": parcel["weight_grams"],
            "dimensions_mm": [parcel["length_mm"], parcel["width_mm"], parcel["height_mm"]],
        }
        response = await self._http.post(RATES_URL, json=payload)
        response.raise_for_status()
        return parse_rates(response.json())

    async def aclose(self) -> None:
        await self._http.aclose()
