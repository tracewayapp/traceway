const FRACTION = /^(.*?)(?:\.(\d+))?(Z|[+-]\d{2}:\d{2})?$/;

// Exact epoch nanoseconds for ordering. preciseTimeMs is floating point and cannot separate spans a few hundred ns apart.
export function timestampNanos(isoString: string): bigint {
	const match = isoString.match(FRACTION);
	const base = Date.parse(match ? `${match[1]}${match[3] ?? ''}` : isoString);
	if (!match || Number.isNaN(base)) return 0n;
	const fraction = (match[2] ?? '').padEnd(9, '0').slice(0, 9);
	return BigInt(base) * 1_000_000n + BigInt(fraction || '0');
}
