import { useEffect, useState } from 'react';
import { OrderHistory } from '../components/OrderHistory.jsx';

export function OrdersPage() {
  const [orders, setOrders] = useState(null);
  useEffect(() => {
    fetch('/api/orders?page=1', { credentials: 'include' })
      .then((r) => r.json())
      .then((body) => setOrders(body.orders));
  }, []);
  if (orders === null) return <p>Loading…</p>;
  return (
    <section>
      <h1>Your orders</h1>
      <OrderHistory orders={orders} />
    </section>
  );
}
