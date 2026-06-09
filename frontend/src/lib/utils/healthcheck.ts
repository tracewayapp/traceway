export const DEFAULT_HEALTHCHECK_PATHS = [
	'/health', '/healthz', '/healthcheck', '/health-check', '/health_check',
	'/ping', '/livez', '/readyz', '/live', '/ready', '/alive', '/up',
	'/heartbeat', '/status', '/ht', '/actuator/health/*', '*/health'
];

const DEFAULT_PATH_SET = new Set([
	'/health', '/healthz', '/healthcheck', '/health-check', '/health_check',
	'/ping', '/livez', '/readyz', '/live', '/ready', '/alive', '/up',
	'/heartbeat', '/status', '/ht', '/actuator/health'
]);

function matchesCustomPath(path: string, pattern: string): boolean {
	if (pattern === '' || pattern === '*') return false;
	const startsWithStar = pattern.startsWith('*');
	const endsWithStar = pattern.endsWith('*');
	if (startsWithStar && endsWithStar) return path.includes(pattern.slice(1, -1));
	if (startsWithStar) return path.endsWith(pattern.slice(1));
	if (endsWithStar) return path.startsWith(pattern.slice(0, -1));
	if (pattern.length > 1) pattern = pattern.replace(/\/+$/, '');
	return path === pattern;
}

export function isHealthcheckEndpoint(endpoint: string, customPaths?: string[] | null): boolean {
	const spaceIdx = endpoint.indexOf(' ');
	if (spaceIdx === -1) return false;
	const method = endpoint.slice(0, spaceIdx);
	if (method !== 'GET' && method !== 'HEAD') return false;
	let path = endpoint.slice(spaceIdx + 1).trim().toLowerCase();
	if (path.length > 1) path = path.replace(/\/+$/, '');
	if (DEFAULT_PATH_SET.has(path)) return true;
	if (path.startsWith('/actuator/health/')) return true;
	if (path.endsWith('/health')) return true;
	for (const custom of customPaths ?? []) {
		if (matchesCustomPath(path, custom.trim().toLowerCase())) return true;
	}
	return false;
}
