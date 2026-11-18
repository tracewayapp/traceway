export type Viewed = { id: number; name: string }

const KEY = 'shop.recentlyViewed'

export function recentlyViewed(): Viewed[] {
  try {
    return JSON.parse(localStorage.getItem(KEY) ?? '[]')
  } catch {
    return []
  }
}

export function rememberViewed(p: Viewed) {
  const rest = recentlyViewed().filter((v) => v.id !== p.id)
  localStorage.setItem(KEY, JSON.stringify([{ id: p.id, name: p.name }, ...rest].slice(0, 8)))
}
