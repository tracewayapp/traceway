'use strict';

const db = require('../db');

// Upserts one product from the supplier feed.
async function upsertFromFeed(entry) {
  await db.query(
    `UPDATE products
        SET name = $2,
            price_cents = $3,
            spec_sheet = $4,
            updated_at = now()
      WHERE slug = $1`,
    [entry.slug, entry.title, Math.round(entry.price * 100), entry.specs],
  );
}

async function importFeed(entries) {
  for (const entry of entries) {
    await upsertFromFeed(entry);
  }
}

module.exports = { importFeed, upsertFromFeed };
