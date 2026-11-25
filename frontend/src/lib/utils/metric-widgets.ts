import type { DashboardWidgetSource, TimeSeriesPoint } from '$lib/types/dashboard';

export const PERCENTILE_AGGREGATIONS = ['p50', 'p75', 'p90', 'p95', 'p99'];
export const GOOD_RATIO = 'good_ratio';

/** Aggregations only a distribution metric (registry type `histogram`) answers. */
export function isDistributionAggregation(aggregation: string | undefined): boolean {
	return (
		!!aggregation && (PERCENTILE_AGGREGATIONS.includes(aggregation) || aggregation === GOOD_RATIO)
	);
}

export const WHOLE_RANGE_WIDGETS = ['rating_bar', 'breakdown_table'];

/**
 * Whether a widget shows one value per series for the whole range. A percentile of the range
 * cannot be put together from the percentiles of its time buckets, so the server computes it.
 */
export function needsWholeRange(
	widgetType: string,
	sources: DashboardWidgetSource[] | undefined
): boolean {
	if (WHOLE_RANGE_WIDGETS.includes(widgetType)) return true;
	if (widgetType !== 'single_value' && widgetType !== 'gauge') return false;
	return isDistributionAggregation(sources?.[0]?.aggregation);
}

function total(points: TimeSeriesPoint[] | undefined): number {
	return (points ?? []).reduce((sum, point) => sum + point.value, 0);
}

export type RatingShare = {
	key: 'good' | 'needs-improvement' | 'poor';
	label: string;
	count: number;
	share: number;
};

const RATINGS: Array<Pick<RatingShare, 'key' | 'label'>> = [
	{ key: 'good', label: 'Good' },
	{ key: 'needs-improvement', label: 'Needs improvement' },
	{ key: 'poor', label: 'Poor' }
];

/** Counts grouped by `rating` as shares of the rated total. Unrated observations are left out. */
export function ratingShares(series: Record<string, TimeSeriesPoint[]>): RatingShare[] {
	const counts = RATINGS.map((rating) => total(series[rating.key]));
	const sum = counts.reduce((a, b) => a + b, 0);
	return RATINGS.map((rating, i) => ({
		...rating,
		count: counts[i],
		share: sum > 0 ? counts[i] / sum : 0
	}));
}

export type BreakdownRow = { key: string; value: number; count: number; share: number };

/** A breakdown of a counter adds up: its value is the total of the range, and that is its weight too. */
export function isAdditiveAggregation(aggregation: string | undefined): boolean {
	return aggregation === 'sum' || aggregation === 'count';
}

/**
 * One row per group, busiest first: what is slow matters in proportion to how many hit it.
 * `additive` is for counters, where the value of a group is the total of its points.
 */
export function breakdownRows(
	values: Record<string, TimeSeriesPoint[]>,
	counts: Record<string, TimeSeriesPoint[]>,
	limit = 10,
	additive = false
): BreakdownRow[] {
	const rows = Object.keys(values).map((key) => {
		const points = values[key] ?? [];
		const sum = total(points);
		return {
			key,
			value: additive ? sum : points.length > 0 ? points[points.length - 1].value : 0,
			count: additive ? sum : total(counts[key]),
			share: 0
		};
	});
	const sum = rows.reduce((a, row) => a + row.count, 0);
	for (const row of rows) row.share = sum > 0 ? row.count / sum : 0;
	return rows
		.sort((a, b) => b.count - a.count || a.key.localeCompare(b.key))
		.slice(0, Math.max(1, limit));
}
