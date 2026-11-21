import type { MetricQueryResponse } from '$lib/types/dashboard';

export const embedPage = '/embed/dashboard';

export type LoadEmbedWidget = (id: string | number) => Promise<MetricQueryResponse>;

// A third-party iframe can have storage disabled. Its only identity comes from the link.
export function dashboardStorage(): Storage | undefined {
	return typeof window !== 'undefined' && window.location.pathname !== embedPage
		? window.localStorage
		: undefined;
}

// Browser timers overflow beyond about 25 days.
export function scheduleEmbedExpiry(seconds: number, expire: () => void) {
	const deadline = Date.now() + seconds * 1000;
	let timer: ReturnType<typeof setTimeout>;
	function check() {
		const remaining = deadline - Date.now();
		if (remaining <= 0) expire();
		else timer = setTimeout(check, Math.min(remaining, 2_147_483_647));
	}
	check();
	return () => clearTimeout(timer);
}
