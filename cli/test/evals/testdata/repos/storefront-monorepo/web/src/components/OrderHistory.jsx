import { orderItemsLabel, orderTotalLabel } from '../lib/orders.js';

function OrderRow({ order }) {
  return (
    <tr>
      <td>{order.id}</td>
      <td>{new Date(order.created_at).toLocaleDateString()}</td>
      <td>{order.status}</td>
      <td>{orderItemsLabel(order)}</td>
      <td>{orderTotalLabel(order)}</td>
    </tr>
  );
}

export function OrderHistory({ orders }) {
  if (orders.length === 0) return <p>You have no orders yet.</p>;
  return (
    <table className="order-history">
      <thead>
        <tr>
          <th>Order</th>
          <th>Date</th>
          <th>Status</th>
          <th>Items</th>
          <th>Total</th>
        </tr>
      </thead>
      <tbody>
        {orders.map((o) => (
          <OrderRow key={o.id} order={o} />
        ))}
      </tbody>
    </table>
  );
}
