import { listProducts } from "@/lib/products";

export const dynamic = "force-dynamic";

export default async function Home() {
  const products = await listProducts();
  return (
    <main>
      <h1>Catalog</h1>
      <ul>
        {products.map((p) => (
          <li key={p.id}>
            <a href={`/api/products/${p.id}`}>{p.name}</a> ${(p.priceCents / 100).toFixed(2)}
            {p.inStock ? "" : " (sold out)"}
          </li>
        ))}
      </ul>
    </main>
  );
}
