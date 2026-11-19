const test = require('node:test');
const assert = require('node:assert/strict');
const { placeOrder } = require('../src/orders/service');
const { createOrderHandler } = require('../src/orders/routes');
const { MemoryOrdersRepo } = require('../src/orders/memoryRepo');

function deps(carts) {
  const charges = [];
  return {
    repo: new MemoryOrdersRepo(carts),
    payments: {
      charge: async (c) => {
        charges.push(c);
        await new Promise((r) => setTimeout(r, 5));
        return { id: `ch_${charges.length}` };
      },
    },
    mailer: { enqueueConfirmation: async () => {} },
    charges,
  };
}

const cart = () => ({ userId: 'u_7', currency: 'EUR', items: [{ sku: 'MUG', qty: 1, unitPriceCents: 1200 }] });

test('a double submit of one cart yields one order and one charge', async () => {
  const d = deps({ c_9: cart() });
  const [a, b] = await Promise.all([
    placeOrder(d, { cartId: 'c_9', paymentMethodId: 'pm_1' }),
    placeOrder(d, { cartId: 'c_9', paymentMethodId: 'pm_1' }),
  ]);
  assert.equal(a.id, b.id, 'both submits must answer the same order');
  assert.equal(d.repo.orders.length, 1);
  assert.equal(d.charges.length, 1, 'the card must be charged once');
});

test('submitting a cart again after it became an order returns that order', async () => {
  const d = deps({ c_8: cart() });
  const first = await placeOrder(d, { cartId: 'c_8', paymentMethodId: 'pm_1' });
  const again = await placeOrder(d, { cartId: 'c_8', paymentMethodId: 'pm_1' });
  assert.equal(again.id, first.id);
  assert.equal(d.charges.length, 1);
});

test('different carts still get their own orders', async () => {
  const d = deps({ c_1: cart(), c_2: cart() });
  const [a, b] = await Promise.all([
    placeOrder(d, { cartId: 'c_1', paymentMethodId: 'pm_1' }),
    placeOrder(d, { cartId: 'c_2', paymentMethodId: 'pm_2' }),
  ]);
  assert.notEqual(a.id, b.id);
  assert.equal(d.charges.length, 2);
});

test('the route answers the duplicate with a success, not a 500', async () => {
  const d = deps({ c_5: cart() });
  const handler = createOrderHandler(d);
  const call = () =>
    new Promise((resolve) => {
      const res = {
        statusCode: 200,
        status(code) { this.statusCode = code; return this; },
        json(body) { resolve({ status: this.statusCode, body }); },
      };
      handler({ body: { cartId: 'c_5', paymentMethodId: 'pm_1' } }, res, (err) => resolve({ status: 500, err }));
    });
  const [a, b] = await Promise.all([call(), call()]);
  for (const r of [a, b]) {
    assert.ok(r.status >= 200 && r.status < 300, `a submit answered ${r.status}: ${r.err?.message ?? ''}`);
  }
  assert.equal(a.body.id, b.body.id);
});
