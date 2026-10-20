const base = import.meta.env.VITE_API_URL ?? "";

export async function fetchProducts() {
  const res = await fetch(`${base}/api/products`);
  if (!res.ok) {
    throw new Error(`GET /api/products failed (${res.status})`);
  }
  return res.json();
}

export async function addToCart(productId) {
  const res = await fetch(`${base}/api/cart/items`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ productId, qty: 1 }),
  });
  if (!res.ok) {
    throw new Error(`POST /api/cart/items failed (${res.status})`);
  }
  return res.json();
}
