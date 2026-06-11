const stockLevels = new Map([["ord_1042", 2]]);

export function reserveStock(orderId, units) {
  const available = stockLevels.get(orderId) ?? 0;
  if (units > available) {
    throw new RangeError(
      `cannot reserve ${units} units for order ${orderId}, only ${available} in stock`,
    );
  }
  stockLevels.set(orderId, available - units);
  return { orderId, units };
}
