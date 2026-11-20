import type { MetricQueryResponse } from '$lib/types/dashboard';

export const embedPage = '/embed/dashboard';

export type LoadEmbedWidget = (id: string | number) => Promise<MetricQueryResponse>;

// A third-party iframe can have storage disabled. Its only identity comes from the link.
export function dashboardStorage(): Storage | undefined {
	return typeof window !== 'undefined' && window.location.pathname === embedPage
		? undefined
		: localStorage;
}
