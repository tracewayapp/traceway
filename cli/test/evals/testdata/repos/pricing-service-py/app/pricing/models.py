from dataclasses import dataclass, field


@dataclass
class Price:
    sku: str
    family: str
    unit_price_cents: int
    tax_class: str


@dataclass
class Promotion:
    id: int
    kind: str  # "volume" | "bundle"
    family: str | None = None
    sku: str | None = None
    partner_sku: str | None = None
    min_quantity: int = 0
    percent: int = 0
    amount_cents: int = 0


@dataclass
class Line:
    sku: str
    family: str
    quantity: int
    unit_price_cents: int
    tax_class: str
    discount_cents: int = 0
    applied: list[int] = field(default_factory=list)

    @property
    def subtotal_cents(self) -> int:
        return self.unit_price_cents * self.quantity


@dataclass
class Quote:
    lines: list[Line]
    subtotal_cents: int
    discount_cents: int
    tax_cents: int

    @property
    def total_cents(self) -> int:
        return self.subtotal_cents - self.discount_cents + self.tax_cents
