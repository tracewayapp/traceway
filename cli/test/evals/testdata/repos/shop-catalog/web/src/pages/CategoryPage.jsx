import { Link, useOutletContext, useParams } from 'react-router';

export function CategoryPage() {
  const { slug } = useParams();
  const { catalog } = useOutletContext();
  if (!catalog) return <p>Loading…</p>;
  const products = slug ? catalog.filter((p) => p.category === slug) : catalog.slice(0, 48);
  return (
    <ul className="grid">
      {products.map((p) => (
        <li key={p.id}>
          <Link to={`/p/${p.sku}`}>
            <img src={p.image_url} alt="" loading="lazy" />
            {p.title} · {(p.amount_minor / 100).toFixed(2)} {p.currency}
          </Link>
        </li>
      ))}
    </ul>
  );
}
