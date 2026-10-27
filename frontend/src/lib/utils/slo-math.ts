import type { SLO, SloSeriesBucket } from '$lib/state/slos.svelte';
import { spanKindLabel } from '$lib/utils/span-fields';

export type SloTone = 'success' | 'warning' | 'danger' | 'neutral';

export interface ChartPoint {
	timestamp: Date;
	value: number;
}

export interface BurnRatePreset {
	id: string;
	label: string;
	longWindowMinutes: number;
	shortWindowMinutes: number;
	burnRateThreshold: number;
}

export const BURN_RATE_PRESETS: BurnRatePreset[] = [
	{
		id: 'fast',
		label: 'Fast',
		longWindowMinutes: 60,
		shortWindowMinutes: 5,
		burnRateThreshold: 14.4
	},
	{
		id: 'slow',
		label: 'Slow',
		longWindowMinutes: 360,
		shortWindowMinutes: 30,
		burnRateThreshold: 6
	},
	{
		id: 'long',
		label: 'Long',
		longWindowMinutes: 4320,
		shortWindowMinutes: 360,
		burnRateThreshold: 1
	}
];

export const SLO_BUCKET_RETENTION_DAYS = 120;

const INTERVAL_STEPS = [1, 5, 15, 30, 60, 120, 240, 480, 720, 1440];

export function sliSeries(buckets: SloSeriesBucket[]): ChartPoint[] {
	return buckets
		.filter((bucket) => bucket.total > 0)
		.map((bucket) => ({
			timestamp: new Date(bucket.bucket),
			value: (bucket.good / bucket.total) * 100
		}));
}

export function errorBudgetSeries(buckets: SloSeriesBucket[], targetPercent: number): ChartPoint[] {
	const total = buckets.reduce((sum, bucket) => sum + bucket.total, 0);
	const allowedBad = (1 - targetPercent / 100) * total;
	if (total === 0 || allowedBad <= 0) return [];
	let bad = 0;
	return buckets.map((bucket) => {
		bad += bucket.total - bucket.good;
		return { timestamp: new Date(bucket.bucket), value: (bad / allowedBad) * 100 };
	});
}

export function budgetTone(remaining: number | null): SloTone {
	if (remaining === null) return 'neutral';
	if (remaining < 0) return 'danger';
	if (remaining < 25) return 'warning';
	return 'success';
}

export function burnTone(rate: number | null): SloTone {
	if (rate === null) return 'neutral';
	if (rate >= BURN_RATE_PRESETS[0].burnRateThreshold) return 'danger';
	if (rate >= 1) return 'warning';
	return 'success';
}

export function toneTextClass(tone: SloTone): string {
	switch (tone) {
		case 'success':
			return 'text-green-600 dark:text-green-400';
		case 'warning':
			return 'text-amber-600 dark:text-amber-400';
		case 'danger':
			return 'text-red-600 dark:text-red-400';
		default:
			return 'text-muted-foreground';
	}
}

export function toneBarClass(tone: SloTone): string {
	switch (tone) {
		case 'success':
			return 'bg-green-500';
		case 'warning':
			return 'bg-amber-500';
		case 'danger':
			return 'bg-red-500';
		default:
			return 'bg-muted-foreground/40';
	}
}

function decimalsOf(value: number): number {
	const text = String(value);
	const dot = text.indexOf('.');
	return dot === -1 ? 0 : text.length - dot - 1;
}

export function formatSloPercent(value: number | null, targetPercent: number): string {
	if (value === null || Number.isNaN(value)) return '—';
	const decimals = Math.max(2, decimalsOf(targetPercent));
	const factor = 10 ** decimals;
	return `${(Math.floor(value * factor + 1e-6) / factor).toFixed(decimals)}%`;
}

export function formatBurnRate(rate: number | null): string {
	if (rate === null) return '—';
	return `${rate.toFixed(rate >= 10 ? 0 : 1)}x`;
}

export function defaultPresetForWindow(windowDays: number): string {
	if (windowDays <= 7) return '7d';
	if (windowDays <= 30) return '1M';
	return '3M';
}

export function clampRangeToWindow(
	from: Date,
	to: Date,
	windowDays: number,
	now: Date
): { from: Date; to: Date; clamped: boolean } {
	const windowMs = windowDays * 24 * 60 * 60 * 1000;
	let clamped = false;
	const clampedTo = to.getTime() > now.getTime() ? now : to;
	let clampedFrom = from;
	const earliest = new Date(clampedTo.getTime() - windowMs);
	if (clampedFrom.getTime() < earliest.getTime()) {
		clampedFrom = earliest;
		clamped = true;
	}
	return { from: clampedFrom, to: clampedTo, clamped };
}

export function calculateSloInterval(from: Date, to: Date, bucketMinutes: number): number {
	const raw = (to.getTime() - from.getTime()) / 60000 / 150;
	for (const step of INTERVAL_STEPS) {
		if (step >= raw - 0.05 && step >= bucketMinutes) return step;
	}
	return 1440;
}

export function sliChartDomain(points: ChartPoint[], targetPercent: number): [number, number] {
	let low = targetPercent;
	for (const point of points) {
		if (point.value < low) low = point.value;
	}
	const pad = Math.max(0.05, (100 - low) * 0.15);
	return [Math.max(0, low - pad), 100];
}

export function budgetChartDomain(points: ChartPoint[]): [number, number] {
	let high = 100;
	for (const point of points) {
		if (point.value > high) high = point.value;
	}
	return [0, Math.min(high, 250) * 1.05];
}

function goodLabel(good: string, latencyThresholdMs: number): string {
	return good === 'latency' ? `latency ≤ ${latencyThresholdMs}ms` : 'availability';
}

export function sliSummary(slo: Pick<SLO, 'sliKind' | 'sliConfig' | 'bucketMinutes'>): string {
	const cfg = slo.sliConfig as unknown as Record<string, unknown>;
	switch (slo.sliKind) {
		case 'endpoint': {
			const endpoint = (cfg.endpoint as string) || 'all endpoints';
			return `${endpoint} · ${goodLabel(cfg.good as string, cfg.latencyThresholdMs as number)}`;
		}
		case 'span': {
			const parts: string[] = [];
			if (cfg.serviceName) parts.push(cfg.serviceName as string);
			if (cfg.name) parts.push(`name ~ ${cfg.name as string}`);
			if (cfg.kind !== null && cfg.kind !== undefined)
				parts.push(spanKindLabel(cfg.kind as number));
			const attributes = (cfg.attributes as SpanSliAttributeLike[] | undefined) ?? [];
			if (attributes.length === 1) parts.push(`${attributes[0].key}=${attributes[0].value}`);
			else if (attributes.length > 1) parts.push(`${attributes.length} attributes`);
			if (parts.length === 0) parts.push('all spans');
			parts.push(goodLabel(cfg.good as string, cfg.latencyThresholdMs as number));
			return parts.join(' · ');
		}
		case 'task':
			return (cfg.taskName as string) || 'all tasks';
		case 'monitor': {
			const ids = (cfg.checkIds as number[] | undefined) ?? [];
			return ids.length === 1 ? '1 monitor' : `${ids.length} monitors`;
		}
		case 'session': {
			const version = (cfg.appVersion as string) || 'all versions';
			return `${version} · crash-free sessions${cfg.includeMessages ? ' (messages count)' : ''}`;
		}
		case 'metric': {
			const operator =
				{ gt: '>', gte: '≥', lt: '<', lte: '≤' }[cfg.operator as string] ?? cfg.operator;
			return `${cfg.aggregation as string}(${cfg.metricName as string}) ${operator} ${cfg.threshold as number} · ${slo.bucketMinutes}m slices`;
		}
	}
	return slo.sliKind;
}

interface SpanSliAttributeLike {
	key: string;
	value: string;
}

export function formatBudgetRemaining(remaining: number | null): string {
	if (remaining === null || Number.isNaN(remaining)) return '—';
	if (remaining <= -100) return `${Math.round(remaining).toLocaleString('en-US')}%`;
	return `${remaining.toFixed(1)}%`;
}
