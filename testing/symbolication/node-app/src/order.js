import { reserveStock } from "./inventory.js";

export function fulfillOrder(order) {
  const reservation = reserveStock(order.orderId, order.units);
  return { orderId: order.orderId, reservation };
}
