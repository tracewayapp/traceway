// Rebuilds search_documents (one row per listing and night, with the
// denormalized fields the new ranking needs) from listings, availability and
// reviews. Runs once as a Kubernetes Job; see deploy/k8s/rebuild-search-documents.yaml.
import pg from 'pg';

const BATCH = 50_000;
const pool = new pg.Pool({ connectionString: process.env.DATABASE_URL, max: 4 });

async function main() {
  await pool.query('CREATE TABLE IF NOT EXISTS search_documents_next (LIKE search_documents INCLUDING ALL)');
  const { rows: [{ max }] } = await pool.query('SELECT max(id) AS max FROM listings');
  for (let from = 0; from <= max; from += BATCH) {
    await pool.query(`
      INSERT INTO search_documents_next (listing_id, day, city, title, price_cents, units_free, rating, review_count, tsv)
      SELECT l.id, a.day, l.city, l.title, a.price_cents, a.units_free,
             coalesce(avg(r.rating), 0), count(r.id),
             to_tsvector('simple', l.title || ' ' || l.city || ' ' || coalesce(string_agg(r.body, ' '), ''))
      FROM listings l
      JOIN availability a ON a.listing_id = l.id
      LEFT JOIN reviews r ON r.listing_id = l.id
      WHERE l.id > $1 AND l.id <= $2
      GROUP BY l.id, a.day, l.city, l.title, a.price_cents, a.units_free`, [from, from + BATCH]);
    console.log(JSON.stringify({ msg: 'batch done', from, to: from + BATCH }));
  }
  await pool.query('BEGIN');
  await pool.query('ALTER TABLE search_documents RENAME TO search_documents_old');
  await pool.query('ALTER TABLE search_documents_next RENAME TO search_documents');
  await pool.query('COMMIT');
  await pool.end();
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
