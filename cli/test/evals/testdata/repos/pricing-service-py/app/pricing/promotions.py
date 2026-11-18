from .models import Line, Promotion


def apply_promotions(lines: list[Line], promotions: list[Promotion]) -> list[Line]:
    """Apply the best eligible promotion to every line.

    volume: a percentage off every line of a product family once the cart
    holds at least min_quantity units of that family.
    bundle: a fixed amount off each unit of `sku` when `partner_sku` is in
    the cart too.
    """
    for line in lines:
        family_quantity = sum(other.quantity for other in lines if other.family == line.family)
        for promo in promotions:
            if promo.kind == "volume" and promo.family == line.family and family_quantity >= promo.min_quantity:
                discount = line.subtotal_cents * promo.percent // 100
            elif promo.kind == "bundle" and promo.sku == line.sku and promo.partner_sku in [l.sku for l in lines]:
                discount = promo.amount_cents * line.quantity
            else:
                continue
            if discount > line.discount_cents:
                line.discount_cents = discount
                line.applied = [promo.id]
    return lines
