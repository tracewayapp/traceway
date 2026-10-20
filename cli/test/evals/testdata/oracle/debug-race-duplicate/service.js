const logger = require('../logger');
const telemetry = require('../telemetry');

class CartNotFoundError extends Error {}
class EmptyCartError extends Error {}

/**
 * Turns a cart into a paid order.
 *
 * @param {object} deps { repo, payments, mailer }
 * @param {{cartId: string, paymentMethodId: string}} input
 */
async function placeOrder({ repo, payments, mailer }, { cartId, paymentMethodId }) {
  telemetry.setAttributes({ 'cart.id': cartId });
  const cart = await repo.findCart(cartId);
  if (!cart) throw new CartNotFoundError(`cart ${cartId} not found`);
  if (cart.items.length === 0) throw new EmptyCartError(`cart ${cartId} is empty`);

  const totalCents = cart.items.reduce((sum, i) => sum + i.qty * i.unitPriceCents, 0);
  let order;
  try {
    order = await repo.insertOrder({ cartId, userId: cart.userId, totalCents, currency: cart.currency });
  } catch (err) {
    if (err.code === '23505' && err.constraint === 'orders_cart_id_key') {
      // The cart was submitted twice (double click, a client retrying a slow
      // request). The cart id is the idempotency key: answer with the order
      // the first submit created instead of failing, and do not charge again.
      const existing = await repo.findOrderByCart(cartId);
      if (existing) {
        logger.info(`cart ${cartId} was submitted again; returning order ${existing.id}`);
        return existing;
      }
    }
    throw err;
  }

  const charge = await payments.charge({ amountCents: totalCents, currency: cart.currency, paymentMethodId, reference: order.id });
  await repo.markPaid(order.id, charge.id);
  await mailer.enqueueConfirmation(order.id);
  logger.info(`order ${order.id} placed for cart ${cartId}`);
  return { ...order, status: 'paid', chargeId: charge.id };
}

module.exports = { placeOrder, CartNotFoundError, EmptyCartError };
