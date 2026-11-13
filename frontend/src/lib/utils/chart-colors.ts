// Categorical series colors from the theme (--chart-1..8 in routes/layout.css),
// in assignment order. Each mode defines its own steps of the same hues.
export const CHART_COLORS = [
	'var(--chart-1)',
	'var(--chart-2)',
	'var(--chart-3)',
	'var(--chart-4)',
	'var(--chart-5)',
	'var(--chart-6)',
	'var(--chart-7)',
	'var(--chart-8)'
] as const;

export function chartColor(index: number): string {
	return CHART_COLORS[index % CHART_COLORS.length];
}
