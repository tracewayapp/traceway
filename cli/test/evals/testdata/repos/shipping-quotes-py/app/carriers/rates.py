from dataclasses import dataclass


@dataclass(frozen=True)
class Rate:
    carrier: str
    service_level: str
    amount_minor: int
    currency: str
    transit_days: int


def parse_rates(body: dict) -> list[Rate]:
    return [
        Rate(
            carrier=r["carrier"],
            service_level=r["service"],
            amount_minor=int(r["amount"]["minor"]),
            currency=r["amount"]["currency"],
            transit_days=int(r["transit_days"]),
        )
        for r in body.get("rates", [])
    ]


def cheapest(rates: list[Rate]) -> Rate | None:
    return min(rates, key=lambda r: (r.amount_minor, r.transit_days), default=None)
