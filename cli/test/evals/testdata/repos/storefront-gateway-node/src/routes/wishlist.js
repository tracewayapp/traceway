'use strict';

async function buildWishlist(customerId, { db, catalog }) {
  const { rows } = await db.query(
    'SELECT sku FROM wishlist_items WHERE customer_id = $1 ORDER BY added_at DESC LIMIT 50',
    [customerId],
  );
  const products = await catalog.getMany(rows.map((r) => r.sku));
  return { items: products.map((p) => ({ sku: p.sku, title: p.title, image: p.imageUrl })) };
}

function wishlistRoute(deps) {
  return async (req, res, next) => {
    try {
      res.json(await buildWishlist(req.session.customerId, deps));
    } catch (err) {
      next(err);
    }
  };
}

module.exports = { buildWishlist, wishlistRoute };
