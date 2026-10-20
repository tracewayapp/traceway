CREATE INDEX IF NOT EXISTS orders_account_id_created_at_idx ON orders (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS order_items_order_id_idx ON order_items (order_id);
