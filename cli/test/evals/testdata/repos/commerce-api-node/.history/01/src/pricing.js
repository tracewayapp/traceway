// Cart and order totals. Prices are integer cents.
export function priceCart(lines) {
  const items = lines.map((l) => ({ id: l.id, name: l.name, qty: l.qty, lineCents: l.price_cents * l.qty }));
  const subtotalCents = items.reduce((s, i) => s + i.lineCents, 0);
  const shippingCents = items.length === 0 ? 0 : 495;
  return { items, subtotalCents, shippingCents, totalCents: subtotalCents + shippingCents };
}
