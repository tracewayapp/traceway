from opentelemetry import trace

from .models import Line, Price, Promotion, Quote
from .promotions import apply_promotions

TAX_RATES = {"standard": 20, "reduced": 10, "zero": 0}

tracer = trace.get_tracer("app.pricing")


class UnknownSku(Exception):
    pass


def build_quote(items: list[dict], prices: dict[str, Price], promotions: list[Promotion]) -> Quote:
    lines = []
    for item in items:
        price = prices.get(item["sku"])
        if price is None:
            raise UnknownSku(item["sku"])
        lines.append(Line(sku=price.sku, family=price.family, quantity=int(item["quantity"]),
                          unit_price_cents=price.unit_price_cents, tax_class=price.tax_class))

    with tracer.start_as_current_span("apply_promotions") as span:
        span.set_attribute("promotions.active", len(promotions))
        apply_promotions(lines, promotions)

    with tracer.start_as_current_span("calculate_tax"):
        tax = sum((l.subtotal_cents - l.discount_cents) * TAX_RATES[l.tax_class] // 100 for l in lines)

    return Quote(lines=lines,
                 subtotal_cents=sum(l.subtotal_cents for l in lines),
                 discount_cents=sum(l.discount_cents for l in lines),
                 tax_cents=tax)
