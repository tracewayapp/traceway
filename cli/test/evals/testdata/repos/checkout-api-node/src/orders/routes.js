const { placeOrder, CartNotFoundError, EmptyCartError } = require('./service');

// POST /api/orders {cartId, paymentMethodId}
function createOrderHandler(deps) {
  return async (req, res, next) => {
    const { cartId, paymentMethodId } = req.body ?? {};
    if (!cartId || !paymentMethodId) {
      return res.status(400).json({ error: 'cartId and paymentMethodId are required' });
    }
    try {
      const order = await placeOrder(deps, { cartId, paymentMethodId });
      res.status(201).json(order);
    } catch (err) {
      if (err instanceof CartNotFoundError) return res.status(404).json({ error: err.message });
      if (err instanceof EmptyCartError) return res.status(422).json({ error: err.message });
      next(err);
    }
  };
}

// GET /api/orders/:id, and GET /api/carts/:id/order for the confirmation page.
function getOrderHandler({ repo }) {
  return async (req, res, next) => {
    try {
      const order = req.params.orderId ? await repo.findOrder(req.params.orderId) : await repo.findOrderByCart(req.params.cartId);
      if (!order) return res.status(404).json({ error: 'order not found' });
      res.json(order);
    } catch (err) {
      next(err);
    }
  };
}

module.exports = { createOrderHandler, getOrderHandler };
