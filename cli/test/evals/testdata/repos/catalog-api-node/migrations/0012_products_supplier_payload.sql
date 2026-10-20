-- Keep the supplier's raw product record (the full feed entry, with its
-- variants, media and compliance documents) so re-imports can diff against
-- it and support can see exactly what the supplier sent.
ALTER TABLE products ADD COLUMN supplier_payload jsonb;
ALTER TABLE products ADD COLUMN supplier_synced_at timestamptz;
