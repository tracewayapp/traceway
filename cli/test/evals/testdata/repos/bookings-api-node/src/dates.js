const DAY = 24 * 60 * 60 * 1000;

export function nightsBetween(checkIn, checkOut) {
  const a = Date.parse(checkIn);
  const b = Date.parse(checkOut);
  if (Number.isNaN(a) || Number.isNaN(b)) return 0;
  return Math.round((b - a) / DAY);
}
