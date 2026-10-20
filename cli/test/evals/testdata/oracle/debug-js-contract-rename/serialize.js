// JSON shapes of orders, shared by the list and detail routes.

function serializeOrderSummary(row) {
  return {
    id: row.id,
    created_at: row.created_at.toISOString(),
    status: row.status,
    item_count: row.item_count,
    total_cents: row.total_cents,
    currency: row.currency,
  };
}

function serializeOrder(row, items) {
  return {
    ...serializeOrderSummary(row),
    shipping_address: row.shipping_address,
    items: items.map((i) => ({ sku: i.sku, name: i.name, qty: i.qty, unit_price_cents: i.unit_price_cents })),
  };
}

module.exports = { serializeOrderSummary, serializeOrder };
