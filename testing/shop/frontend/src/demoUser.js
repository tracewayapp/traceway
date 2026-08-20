const PERSONAS = [
  { name: 'Mia Chen', email: 'mia.chen@example.com', plan: 'free' },
  { name: 'Liam Novak', email: 'liam.novak@example.com', plan: 'pro' },
  { name: 'Sofia Ricci', email: 'sofia.ricci@example.com', plan: 'free' },
  { name: 'Noah Fischer', email: 'noah.fischer@example.com', plan: 'pro' },
  { name: 'Emma Kovacs', email: 'emma.kovacs@example.com', plan: 'free' },
  { name: 'Lucas Moreau', email: 'lucas.moreau@example.com', plan: 'enterprise' },
  { name: 'Olivia Jansen', email: 'olivia.jansen@example.com', plan: 'free' },
  { name: 'Ethan Silva', email: 'ethan.silva@example.com', plan: 'pro' },
  { name: 'Ava Lindberg', email: 'ava.lindberg@example.com', plan: 'free' },
  { name: 'Marko Petrovic', email: 'marko.petrovic@example.com', plan: 'pro' },
  { name: 'Hana Sato', email: 'hana.sato@example.com', plan: 'free' },
  { name: 'Jonas Weber', email: 'jonas.weber@example.com', plan: 'enterprise' }
]

export function getDemoUser() {
  if (window.__DEMO_USER) return window.__DEMO_USER
  try {
    const stored = sessionStorage.getItem('shoply_demo_user')
    if (stored) return JSON.parse(stored)
    const user = PERSONAS[Math.floor(Math.random() * PERSONAS.length)]
    sessionStorage.setItem('shoply_demo_user', JSON.stringify(user))
    return user
  } catch {
    return PERSONAS[0]
  }
}
