const test = require('node:test');
const assert = require('node:assert/strict');
const { placeOrder } = require('../src/orders/service');
const { MemoryOrdersRepo } = require('../src/orders/memoryRepo');

function deps(carts) {
  const charges = [];
  const mails = [];
  return {
    repo: new MemoryOrdersRepo(carts),
    payments: { charge: async (c) => { charges.push(c); return { id: `ch_${charges.length}` }; } },
    mailer: { enqueueConfirmation: async (id) => { mails.push(id); } },
    charges,
    mails,
  };
}

const cart = { userId: 'u_1', currency: 'USD', items: [{ sku: 'TEE-M', qty: 2, unitPriceCents: 1500 }] };

test('places and charges an order', async () => {
  const d = deps({ c_1: cart });
  const order = await placeOrder(d, { cartId: 'c_1', paymentMethodId: 'pm_1' });
  assert.equal(order.status, 'paid');
  assert.equal(order.totalCents, 3000);
  assert.equal(d.charges.length, 1);
  assert.deepEqual(d.mails, [order.id]);
});
