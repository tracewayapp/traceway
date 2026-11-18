'use strict';

// A product card is what the grid and search results render.
function toCard(p) {
  return {
    id: p.id,
    name: p.name,
    slug: p.slug,
    priceCents: p.price_cents,
    currency: p.currency,
    thumbnailUrl: p.thumbnail_url,
  };
}

module.exports = { toCard };
