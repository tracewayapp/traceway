import { formatCents } from './money.js';

// Labels for the order history table, from GET /api/orders summaries.
export function orderTotalLabel(order) {
  return formatCents(order.total_cents, order.currency);
}

export function orderItemsLabel(order) {
  return order.item_count === 1 ? '1 item' : `${order.item_count} items`;
}
