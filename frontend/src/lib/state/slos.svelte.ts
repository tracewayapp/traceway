import { api } from '$lib/api';
import { projectsState } from '$lib/state/projects.svelte';
import type { NotificationRuleConfig } from '$lib/types/notifications';

export type SliKind = 'endpoint' | 'span' | 'task' | 'monitor' | 'metric' | 'session';
export type SloStatus = 'ok' | 'breached' | 'no_data';
export type GoodCondition = 'availability' | 'latency';
export type MetricAggregation = 'avg' | 'min' | 'max' | 'sum' | 'last';
export type MetricOperator = 'gt' | 'gte' | 'lt' | 'lte';

export interface EndpointSliConfig {
	endpoint: string;
	good: GoodCondition;
	latencyThresholdMs: number;
}

export interface SpanSliAttribute {
	key: string;
	value: string;
}

export interface SpanSliConfig {
	serviceName: string;
	name: string;
	kind: number | null;
	attributes: SpanSliAttribute[];
	good: GoodCondition;
	latencyThresholdMs: number;
}

export interface TaskSliConfig {
	taskName: string;
}

export interface MonitorSliConfig {
	checkIds: number[];
}

export interface MetricSliConfig {
	metricName: string;
	tags: Record<string, string>;
	aggregation: MetricAggregation;
	operator: MetricOperator;
	threshold: number | null;
	missingIsBad: boolean;
}

export interface SessionSliConfig {
	appVersion: string;
	includeMessages: boolean;
}

export type SliConfig =
	| EndpointSliConfig
	| SpanSliConfig
	| TaskSliConfig
	| MonitorSliConfig
	| MetricSliConfig
	| SessionSliConfig;

export interface SLO {
	id: number;
	projectId: string;
	name: string;
	description: string;
	sliKind: SliKind;
	sliConfig: SliConfig;
	targetPercent: number;
	windowDays: number;
	bucketMinutes: number;
	enabled: boolean;
	status: SloStatus;
	currentSli: number | null;
	budgetRemaining: number | null;
	burnRate1h: number | null;
	burnRate6h: number | null;
	computedAt: string | null;
	historyFrom: string | null;
	lastBucketAt: string | null;
	createdAt: string;
	updatedAt: string;
}

export interface SloSeriesBucket {
	bucket: string;
	good: number;
	total: number;
}

export interface SloPreview {
	good: number;
	total: number;
	sli: number | null;
	from: string;
	to: string;
}

export interface SloAlertRule {
	id: number;
	name: string;
	ruleType: string;
	config: NotificationRuleConfig;
	enabled: boolean;
	snoozedUntil: string | null;
	channelName: string;
	channelType: string;
}

export const SLI_KIND_LABELS: Record<SliKind, string> = {
	endpoint: 'Endpoint',
	span: 'Span',
	task: 'Task',
	monitor: 'Monitor',
	metric: 'Metric',
	session: 'Session'
};

export const BACKEND_SLI_KINDS: SliKind[] = ['endpoint', 'span', 'task', 'monitor', 'metric'];
export const FRONTEND_SLI_KINDS: SliKind[] = ['session'];

export const SLO_WINDOW_OPTIONS = [
	{ value: '7', label: '7 days' },
	{ value: '30', label: '30 days' },
	{ value: '90', label: '90 days' }
];

class SlosState {
	breachedCount = $state(0);

	async refreshBreachedCount() {
		const res = await api
			.get('/slos/breached-count', { projectId: projectsState.currentProjectId ?? undefined })
			.catch(() => null);
		if (res) this.breachedCount = res.count ?? 0;
	}
}

export const slosState = new SlosState();
