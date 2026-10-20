import { useEffect, useState } from "react";
import { addToCart, fetchProducts } from "./api.js";

export default function App() {
  const [products, setProducts] = useState([]);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetchProducts().then(setProducts, (e) => setError(e.message));
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
            {p.name} <button onClick={() => addToCart(p.id)}>Add to cart</button>
          </li>
        ))}
      </ul>
    </main>
  );
}
