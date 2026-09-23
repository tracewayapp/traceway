let resolved: Promise<string | null> | null = null;

export function resolveStatusDomain(fetchFn: typeof fetch = fetch): Promise<string | null> {
	resolved ??= fetchFn('/api/status-domains/resolve')
		.then((response) => (response.ok ? response.json() : null))
		.then((body) => (typeof body?.slug === 'string' && body.slug ? body.slug : null))
		.catch(() => null);
	return resolved;
}
