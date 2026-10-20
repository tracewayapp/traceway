'use strict';
const test = require('node:test');
const assert = require('node:assert');
const { buildCart } = require('../src/routes/cart');

test('buildCart prices every line and totals the cart', async () => {
  const clients = {
    carts: { get: async () => ({ id: 'c1', lines: [{ sku: 'A', qty: 2 }, { sku: 'B', qty: 1 }] }) },
    catalog: { get: async (sku) => ({ sku, title: `T${sku}`, brand: 'x', imageUrl: `/${sku}.jpg` }) },
    pricing: { get: async (sku, currency) => ({ sku, amountMinor: sku === 'A' ? 500 : 1200, currency }) },
    inventory: { get: async (sku) => ({ sku, available: 3 }) },
  };
  const cart = await buildCart('c1', 'EUR', clients);
  assert.equal(cart.lines.length, 2);
  assert.equal(cart.total, 2200);
  assert.equal(cart.lines[0].inStock, true);
});
