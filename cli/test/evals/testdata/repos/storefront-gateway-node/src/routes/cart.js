'use strict';

// buildCart returns the cart page model: every line with its product,
// price and availability.
async function buildCart(cartId, currency, { carts, catalog, pricing, inventory }) {
  const cart = await carts.get(cartId);
  const lines = [];
  for (const line of cart.lines) {
    const product = await catalog.get(line.sku);
    const price = await pricing.get(line.sku, currency);
    const stock = await inventory.get(line.sku);
    lines.push({
      sku: line.sku,
      qty: line.qty,
      title: product.title,
      brand: product.brand,
      image: product.imageUrl,
      unitPrice: price.amountMinor,
      currency: price.currency,
      inStock: stock.available >= line.qty,
    });
  }
  const total = lines.reduce((sum, l) => sum + l.unitPrice * l.qty, 0);
  return { id: cart.id, lines, total, currency };
}

function cartRoute(clients) {
  return async (req, res, next) => {
    try {
      res.json(await buildCart(req.session.cartId, req.query.currency || 'EUR', clients));
    } catch (err) {
      next(err);
    }
  };
}

module.exports = { buildCart, cartRoute };
