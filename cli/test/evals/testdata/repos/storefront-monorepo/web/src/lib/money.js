const SYMBOLS = { USD: '$', EUR: '€', GBP: '£' };

// Formats integer cents without going through floating point.
export function formatCents(cents, currency = 'USD') {
  const digits = cents.toString().padStart(3, '0');
  const whole = digits.slice(0, -2).replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  return `${SYMBOLS[currency] ?? `${currency} `}${whole}.${digits.slice(-2)}`;
}
