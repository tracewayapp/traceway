const test = require('node:test');
const assert = require('node:assert/strict');
const { serializeOrderSummary, serializeOrder } = require('../api/src/orders/serialize');

const row = { id: 'ord_41', created_at: new Date('2026-03-02T10:00:00Z'), status: 'shipped', item_count: 3, total_cents: 12999, currency: 'USD', shipping_address: { city: 'Austin' } };

test('order summaries keep the snake_case total_cents field the web and mobile clients read', () => {
  const s = serializeOrderSummary(row);
  assert.equal(s.total_cents, 12999);
  assert.equal(s.created_at, '2026-03-02T10:00:00.000Z');
  assert.equal(s.item_count, 3);
  assert.equal(serializeOrder(row, []).total_cents, 12999);
});

test('the order history renders what GET /api/orders serves', async () => {
  const { orderTotalLabel } = await import('../web/src/lib/orders.js');
  assert.equal(orderTotalLabel(serializeOrderSummary(row)), '$129.99');
});
