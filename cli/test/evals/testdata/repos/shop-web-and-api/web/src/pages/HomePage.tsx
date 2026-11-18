import { Link } from 'react-router-dom'
import { recentlyViewed } from '../recentlyViewed'

export function HomePage() {
  const viewed = recentlyViewed()
  return (
    <main>
      <h1>Shop</h1>
      <Link to="/search">Search products</Link>
      {viewed.length > 0 && (
        <section>
          <h2>Recently viewed</h2>
          <ul>{viewed.map((p) => <li key={p.id}><Link to={`/products/${p.id}`}>{p.name}</Link></li>)}</ul>
        </section>
      )}
    </main>
  )
}
