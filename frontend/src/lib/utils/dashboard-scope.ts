import { api } from '$lib/api';
import { valueOptions, type FilterOption } from '$lib/components/traceway/filter-bar';

// The values one tag takes in a metric. A metric without data, or a failed read, offers none.
export async function loadTagValues(
	projectId: string | undefined,
	metric: string,
	key: string
): Promise<string[]> {
	try {
		const response = await api.get(
			`/metrics/discover/tags?name=${encodeURIComponent(metric)}&key=${encodeURIComponent(key)}`,
			{ projectId }
		);
		return response.values ?? [];
	} catch {
		return [];
	}
}

// The values a client dimension (release, OS, country) takes in any of `metrics`, newest
// release first.
export async function loadDimensionValues(
	projectId: string,
	metrics: readonly string[],
	dimension: string
): Promise<FilterOption[]> {
	const lists = await Promise.all(
		metrics.map((metric) => loadTagValues(projectId, metric, dimension))
	);
	const found = new Set(lists.flat());
	return valueOptions([...found].sort((a, b) => b.localeCompare(a, undefined, { numeric: true })));
}

// The instances (`server_name`) that reported in the range.
export async function loadInstances(projectId: string, from: string, to: string) {
	const response = await api.get(
		`/metrics/discover/instances?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`,
		{ projectId }
	);
	const discovered: unknown[] = Array.isArray(response.instances) ? response.instances : [];
	const names = discovered
		.filter((v): v is string => typeof v === 'string' && v.trim() !== '')
		.map((v) => v.trim());
	return valueOptions([...new Set(names)].sort((a, b) => a.localeCompare(b)));
}
