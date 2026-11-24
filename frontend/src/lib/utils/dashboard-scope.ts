import { api } from '$lib/api';
import type { FilterOption } from '$lib/components/traceway/filter-bar';

const asOptions = (values: string[]): FilterOption[] =>
	values.map((value) => ({ value, label: value }));

// The values a client dimension (release, OS, country) takes in any of `metrics`, newest
// release first. A metric without data offers nothing.
export async function loadDimensionValues(
	projectId: string,
	metrics: readonly string[],
	dimension: string
): Promise<FilterOption[]> {
	const found = new Set<string>();
	await Promise.all(
		metrics.map(async (metric) => {
			try {
				const response = await api.get(
					`/metrics/discover/tags?name=${encodeURIComponent(metric)}&key=${encodeURIComponent(dimension)}`,
					{ projectId }
				);
				for (const value of response.values ?? []) found.add(value);
			} catch {
				// Nothing to offer from this metric.
			}
		})
	);
	return asOptions([...found].sort((a, b) => b.localeCompare(a, undefined, { numeric: true })));
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
	return asOptions([...new Set(names)].sort((a, b) => a.localeCompare(b)));
}
