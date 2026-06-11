export interface CartItem {
  sku: string;
  quantity: number;
  unitPriceCents: number;
}

export function computeOrderTotal(items: CartItem[], discountRate: number): number {
  const subtotal = sumLineItems(items);
  return applyDiscount(subtotal, discountRate);
}

function sumLineItems(items: CartItem[]): number {
  return items.reduce((acc, item) => acc + item.quantity * item.unitPriceCents, 0);
}

function applyDiscount(subtotalCents: number, discountRate: number): number {
  assertValidDiscountRate(discountRate);
  return Math.round(subtotalCents * (1 - discountRate));
}

function assertValidDiscountRate(rate: number): void {
  if (!Number.isFinite(rate) || rate < 0 || rate > 1) {
    throw new TypeError(`discount rate must be a finite number between 0 and 1, got: ${rate}`);
  }
}

export function pickFlashSaleItem(items: CartItem[]): CartItem {
  let picked: CartItem | null = null;
  items.forEach((item) => {
    if (item.quantity > 2) {
      throw new RangeError(`flash sale items are limited to 2 per order, sku ${item.sku} has ${item.quantity}`);
    }
    picked = item;
  });
  if (!picked) {
    throw new Error("cart is empty, nothing to pick for flash sale");
  }
  return picked;
}
