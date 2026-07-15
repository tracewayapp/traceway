export type ThresholdStep = { value: number; color: string };

export const THRESHOLD_COLORS: Record<string, string> = {
	green: 'var(--ok)',
	yellow: '#eab308',
	orange: '#f97316',
	red: 'var(--crit)',
	blue: 'var(--blue)'
};

export const THRESHOLD_COLOR_NAMES = Object.keys(THRESHOLD_COLORS);

export function resolveThresholdColor(color: string): string {
	return THRESHOLD_COLORS[color] ?? color;
}

export function sortThresholdSteps(steps: ThresholdStep[] | undefined): ThresholdStep[] {
	return [...(steps ?? [])].sort((a, b) => a.value - b.value);
}

export function activeThresholdColor(
	value: number,
	baseColor: string | undefined,
	steps: ThresholdStep[] | undefined
): string {
	let color = baseColor || 'green';
	for (const step of sortThresholdSteps(steps)) {
		if (value >= step.value) color = step.color;
	}
	return resolveThresholdColor(color);
}

export function thresholdBandSegments(
	min: number,
	max: number,
	baseColor: string | undefined,
	steps: ThresholdStep[] | undefined
): Array<{ from: number; to: number; color: string }> {
	const segments: Array<{ from: number; to: number; color: string }> = [];
	let cursor = min;
	let color = baseColor || 'green';
	for (const step of sortThresholdSteps(steps)) {
		const bounded = Math.min(Math.max(step.value, min), max);
		if (bounded > cursor) {
			segments.push({ from: cursor, to: bounded, color: resolveThresholdColor(color) });
			cursor = bounded;
		}
		color = step.color;
	}
	if (cursor < max) {
		segments.push({ from: cursor, to: max, color: resolveThresholdColor(color) });
	}
	return segments;
}
