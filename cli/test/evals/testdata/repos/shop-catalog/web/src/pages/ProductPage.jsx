import { useEffect, useState } from 'react';
import { useOutletContext, useParams } from 'react-router';

export function ProductPage() {
  const { sku } = useParams();
  const { catalog } = useOutletContext();
  const [reviews, setReviews] = useState([]);
  useEffect(() => {
    fetch(`/api/products/${encodeURIComponent(sku)}/reviews?page=1`)
      .then((res) => res.json())
      .then((body) => setReviews(body.reviews));
  }, [sku]);
  const product = catalog?.find((p) => p.sku === sku);
  if (!product) return <p>Loading…</p>;
  return (
    <article>
      <h1>{product.title}</h1>
      <p>{(product.amount_minor / 100).toFixed(2)} {product.currency}</p>
      <section>{reviews.map((r) => <blockquote key={r.id}>{r.title}</blockquote>)}</section>
    </article>
  );
}
