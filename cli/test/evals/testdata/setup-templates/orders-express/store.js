const seedOrders = [
  { id: "1001", customerId: "c-17", status: "shipped", items: [{ sku: "MUG-01", qty: 2 }], total: 24.0 },
  { id: "1002", customerId: "c-17", status: "pending", items: [{ sku: "TEE-03", qty: 1 }], total: 19.5 },
  { id: "1003", customerId: "c-42", status: "pending", items: [{ sku: "CAP-02", qty: 3 }], total: 45.0 },
];

const prices = { "MUG-01": 12.0, "TEE-03": 19.5, "CAP-02": 15.0 };

export function createStore() {
  const orders = new Map(seedOrders.map((o) => [o.id, structuredClone(o)]));
  let nextId = 1004;
  return {
    list({ status } = {}) {
      const all = [...orders.values()];
      return status ? all.filter((o) => o.status === status) : all;
    },
    get(id) {
      return orders.get(String(id)) ?? null;
    },
    create({ customerId, items }) {
      const total = items.reduce((sum, i) => sum + (prices[i.sku] ?? 0) * i.qty, 0);
      const order = { id: String(nextId++), customerId, status: "pending", items, total };
      orders.set(order.id, order);
      return order;
    },
  };
}
