export const PERSONAS = [
  { name: 'Mia Chen', email: 'mia.chen@example.com', plan: 'free', card: '4242', thinkScale: 1.0 },
  { name: 'Liam Novak', email: 'liam.novak@example.com', plan: 'pro', card: '1881', thinkScale: 0.7 },
  { name: 'Sofia Ricci', email: 'sofia.ricci@example.com', plan: 'free', card: '5100', thinkScale: 1.4 },
  { name: 'Noah Fischer', email: 'noah.fischer@example.com', plan: 'pro', card: '4012', thinkScale: 0.8 },
  { name: 'Emma Kovacs', email: 'emma.kovacs@example.com', plan: 'free', card: '9424', thinkScale: 1.2 },
  { name: 'Lucas Moreau', email: 'lucas.moreau@example.com', plan: 'enterprise', card: '7710', thinkScale: 0.9 },
  { name: 'Olivia Jansen', email: 'olivia.jansen@example.com', plan: 'free', card: '3056', thinkScale: 1.1 },
  { name: 'Ethan Silva', email: 'ethan.silva@example.com', plan: 'pro', card: '6011', thinkScale: 0.6 },
  { name: 'Ava Lindberg', email: 'ava.lindberg@example.com', plan: 'free', card: '3530', thinkScale: 1.3 },
  { name: 'Marko Petrovic', email: 'marko.petrovic@example.com', plan: 'pro', card: '2223', thinkScale: 0.9 },
  { name: 'Hana Sato', email: 'hana.sato@example.com', plan: 'free', card: '4917', thinkScale: 1.0 },
  { name: 'Jonas Weber', email: 'jonas.weber@example.com', plan: 'enterprise', card: '8171', thinkScale: 0.8 }
]

export const VIEWPORTS = [
  { width: 1440, height: 900 },
  { width: 1280, height: 800 },
  { width: 1536, height: 960 },
  { width: 834, height: 1112 },
  { width: 390, height: 844 }
]

export function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)]
}
