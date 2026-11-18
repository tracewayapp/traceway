import { useState } from 'react'
import { useTraceway } from '@tracewayapp/react'
import { searchProducts, type SearchHit } from '../api/search'

export function SearchPage() {
  const { captureException } = useTraceway()
  const [hits, setHits] = useState<SearchHit[]>([])
  const [state, setState] = useState<'idle' | 'loading' | 'slow' | 'error'>('idle')

  async function runSearch(q: string) {
    setState('loading')
    try {
      setHits(await searchProducts(q))
      setState('idle')
    } catch (err) {
      if (err instanceof DOMException && err.name === 'TimeoutError') {
        console.warn(`search for "${q}" gave up after 8000ms`)
        setState('slow')
      } else {
        setState('error')
      }
      captureException(err as Error)
    }
  }

  return (
    <main>
      <form onSubmit={(e) => { e.preventDefault(); runSearch(new FormData(e.currentTarget).get('q') as string) }}>
        <input name="q" placeholder="Search products" />
      </form>
      {state === 'slow' && <p>Search is taking too long. Please try again.</p>}
      {state === 'error' && <p>Search is unavailable right now.</p>}
      <ul>{hits.map((h) => <li key={h.id}>{h.name}</li>)}</ul>
    </main>
  )
}
