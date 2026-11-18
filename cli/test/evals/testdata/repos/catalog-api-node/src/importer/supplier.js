'use strict';

const db = require('../db');

// Upserts one product from the supplier feed. The raw feed entry is stored
// as-is in supplier_payload so the next import can diff against it.
async function upsertFromFeed(entry) {
  await db.query(
    `UPDATE products
        SET name = $2,
            price_cents = $3,
            spec_sheet = $4,
            supplier_payload = $5,
            supplier_synced_at = now(),
            updated_at = now()
      WHERE slug = $1`,
    [entry.slug, entry.title, Math.round(entry.price * 100), entry.specs, entry],
  );
}

async function importFeed(entries) {
  for (const entry of entries) {
    await upsertFromFeed(entry);
  }
}

module.exports = { importFeed, upsertFromFeed };
