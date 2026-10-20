import test from 'node:test';
import assert from 'node:assert/strict';
import { formatCents } from '../src/lib/money.js';
import { orderItemsLabel } from '../src/lib/orders.js';

test('formats cents', () => {
  assert.equal(formatCents(129999, 'USD'), '$1,299.99');
  assert.equal(formatCents(5, 'EUR'), '€0.05');
  assert.equal(formatCents(1000, 'CHF'), 'CHF 10.00');
});

test('labels item counts', () => {
  assert.equal(orderItemsLabel({ item_count: 1 }), '1 item');
  assert.equal(orderItemsLabel({ item_count: 3 }), '3 items');
});
