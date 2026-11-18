import { test } from 'node:test';
import assert from 'node:assert/strict';
import { priceCart } from './pricing.js';

test('free shipping from 50 EUR', () => {
  assert.equal(priceCart([{ id: 1, name: 'Mug', price_cents: 2500, qty: 2 }]).shippingCents, 0);
  assert.equal(priceCart([{ id: 1, name: 'Mug', price_cents: 2500, qty: 1 }]).shippingCents, 495);
});

test('totals add up', () => {
  const c = priceCart([{ id: 1, name: 'Mug', price_cents: 1200, qty: 3 }, { id: 2, name: 'Tea', price_cents: 800, qty: 1 }]);
  assert.deepEqual([c.subtotalCents, c.totalCents], [4400, 4895]);
});
