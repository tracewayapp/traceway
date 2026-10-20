const products = [
  { id: "1", name: "Enamel mug", priceCents: 1200, inStock: true },
  { id: "2", name: "Logo tee", priceCents: 1950, inStock: true },
  { id: "3", name: "Dad cap", priceCents: 1500, inStock: false },
];

export async function listProducts() {
  return products;
}

export async function findProduct(id) {
  return products.find((p) => p.id === id) ?? null;
}
