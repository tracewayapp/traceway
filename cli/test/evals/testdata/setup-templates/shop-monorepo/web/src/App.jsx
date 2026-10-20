import { useEffect, useState } from "react";

export default function App() {
  const [products, setProducts] = useState([]);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetch("/api/products")
      .then((res) => {
        if (!res.ok) throw new Error(`GET /api/products failed (${res.status})`);
        return res.json();
      })
      .then(setProducts, (e) => setError(e.message));
  }, []);

  if (error) {
    return <p role="alert">Could not load products: {error}</p>;
  }

  return (
    <main>
      <h1>Shop</h1>
      <ul>
        {products.map((p) => (
          <li key={p.id}>
            {p.name} ${(p.priceCents / 100).toFixed(2)}
          </li>
        ))}
      </ul>
    </main>
  );
}
