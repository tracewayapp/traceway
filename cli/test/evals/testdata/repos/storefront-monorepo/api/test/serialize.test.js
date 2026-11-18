const test = require('node:test');
const assert = require('node:assert/strict');
const { serializeOrder } = require('../src/orders/serialize');

test('order detail includes its items', () => {
  const row = { id: 'ord_7', created_at: new Date('2026-03-02T10:00:00Z'), status: 'shipped', item_count: 1, total_cents: 2500, currency: 'USD', shipping_address: { city: 'Austin' } };
  const out = serializeOrder(row, [{ sku: 'MUG', name: 'Mug', qty: 1, unit_price_cents: 2500 }]);
  assert.equal(out.id, 'ord_7');
  assert.equal(out.created_at, '2026-03-02T10:00:00.000Z');
  assert.deepEqual(out.items, [{ sku: 'MUG', name: 'Mug', qty: 1, unit_price_cents: 2500 }]);
});
