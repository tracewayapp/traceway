import type { SLO, SpanSliConfig } from '$lib/state/slos.svelte';
import { spanSearchUrlParams } from '$lib/utils/span-explorer';

const BAD_EVENTS_PRESET = '24h';

export function badEventsLabel(slo: Pick<SLO, 'sliKind'>): string {
	switch (slo.sliKind) {
		case 'endpoint':
			return 'View requests';
		case 'span':
			return 'View bad spans';
		case 'task':
			return 'View task runs';
		case 'monitor':
			return 'Open monitor';
		case 'session':
			return 'View sessions';
		default:
			return 'Open dashboards';
	}
}

export function badEventsHref(slo: Pick<SLO, 'sliKind' | 'sliConfig'>): string {
	const cfg = slo.sliConfig as unknown as Record<string, unknown>;
	switch (slo.sliKind) {
		case 'endpoint': {
			const endpoint = cfg.endpoint as string;
			return endpoint
				? `/endpoints/${encodeURIComponent(endpoint)}?preset=${BAD_EVENTS_PRESET}`
				: `/endpoints?preset=${BAD_EVENTS_PRESET}`;
		}
		case 'span': {
			const span = slo.sliConfig as SpanSliConfig;
			const params = spanSearchUrlParams({
				name: span.name ?? '',
				service: span.serviceName ?? '',
				kind: span.kind === null || span.kind === undefined ? '' : String(span.kind),
				status: span.good === 'availability' ? '2' : '',
				minMs: span.good === 'latency' ? String(span.latencyThresholdMs) : '',
				maxMs: '',
				traceId: '',
				attributes: span.attributes ?? []
			});
			const search = new URLSearchParams();
			for (const [key, value] of Object.entries(params)) {
				if (value === undefined) continue;
				if (Array.isArray(value)) {
					for (const item of value) search.append(key, item);
				} else {
					search.set(key, value);
				}
			}
			search.set('preset', BAD_EVENTS_PRESET);
			return `/spans?${search.toString()}`;
		}
		case 'task': {
			const taskName = cfg.taskName as string;
			return taskName
				? `/tasks/${encodeURIComponent(taskName)}?preset=${BAD_EVENTS_PRESET}`
				: `/tasks?preset=${BAD_EVENTS_PRESET}`;
		}
		case 'session':
			return `/sessions?preset=${BAD_EVENTS_PRESET}`;
		case 'monitor': {
			const ids = (cfg.checkIds as number[] | undefined) ?? [];
			return ids.length === 1 ? `/monitors/${ids[0]}` : '/monitors';
		}
		default:
			return '/dashboards';
	}
}
