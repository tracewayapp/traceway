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
  const order = await repo.insertOrder({ cartId, userId: cart.userId, totalCents, currency: cart.currency });

  const charge = await payments.charge({ amountCents: totalCents, currency: cart.currency, paymentMethodId, reference: order.id });
  await repo.markPaid(order.id, charge.id);
  await mailer.enqueueConfirmation(order.id);
  logger.info(`order ${order.id} placed for cart ${cartId}`);
  return { ...order, status: 'paid', chargeId: charge.id };
}

module.exports = { placeOrder, CartNotFoundError, EmptyCartError };
