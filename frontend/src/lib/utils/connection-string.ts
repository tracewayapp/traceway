// Connection strings (`<token>@<server>`) and server URLs, read with the rules every Traceway SDK uses
//, so a value from any SDK version, old env file or old plan names the same server here.

const SCHEME = /^[A-Za-z][A-Za-z0-9+.-]*:\/\//;
const PLAIN_HTTP_HOSTS = new Set(['localhost', '127.0.0.1', '10.0.2.2', '[::1]']);

function hostWithoutPort(s: string): string {
	const end = s.search(/[/?#]/);
	const authority = end >= 0 ? s.slice(0, end) : s;
	if (authority.startsWith('[')) {
		const close = authority.indexOf(']');
		return close >= 0 ? authority.slice(0, close + 1) : authority;
	}
	const colon = authority.lastIndexOf(':');
	return colon >= 0 ? authority.slice(0, colon) : authority;
}

/**
 * The base URL for any spelling of a server URL: `https://` added when there is no scheme (`http://` for
 * localhost, 127.0.0.1, 10.0.2.2 and [::1]), the query, fragment, trailing slashes and a trailing `/api/report` or
 * `/api` dropped. Any other path is kept. Null when the result is not an http(s) URL with a host.
 */
export function normalizeBaseUrl(raw: string | null | undefined): string | null {
	let s = (raw ?? '').trim();
	if (!s) return null;
	if (!SCHEME.test(s)) {
		s = (PLAIN_HTTP_HOSTS.has(hostWithoutPort(s).toLowerCase()) ? 'http://' : 'https://') + s;
	}
	const cut = s.search(/[?#]/);
	if (cut >= 0) s = s.slice(0, cut);
	s = s.replace(/\/+$/, '');
	if (s.endsWith('/api/report')) s = s.slice(0, -'/api/report'.length);
	else if (s.endsWith('/api')) s = s.slice(0, -'/api'.length);
	s = s.replace(/\/+$/, '');

	try {
		const url = new URL(s);
		if ((url.protocol !== 'http:' && url.protocol !== 'https:') || !url.host) return null;
	} catch {
		return null;
	}
	return s;
}

/** The token and base URL of a connection string, or null when it is not one. */
export function parseConnectionString(
	raw: string | null | undefined
): { token: string; baseUrl: string } | null {
	const s = (raw ?? '').trim();
	const at = s.indexOf('@');
	if (at < 0) return null;
	const token = s.slice(0, at).trim();
	if (!token) return null;
	const baseUrl = normalizeBaseUrl(s.slice(at + 1));
	return baseUrl ? { token, baseUrl } : null;
}

/**
 * Any spelling of a connection string as `<token>@<baseUrl>`, which every 2.x SDK reads the same way, or null when
 * it is not one.
 */
export function normalizeConnectionString(raw: string | null | undefined): string | null {
	const parsed = parseConnectionString(raw);
	return parsed ? `${parsed.token}@${parsed.baseUrl}` : null;
}

/** Whether a server URL is Traceway Cloud, where the SDKs need no `baseUrl`. */
export function isCloudUrl(raw: string | null | undefined): boolean {
	const base = normalizeBaseUrl(raw);
	if (!base) return false;
	const url = new URL(base);
	return (
		url.protocol === 'https:' &&
		url.hostname.toLowerCase() === 'cloud.tracewayapp.com' &&
		(url.port === '' || url.port === '443') &&
		url.pathname.replace(/\/+$/, '') === ''
	);
}

/** The server URL to show in SDK snippets: null on Traceway Cloud, the server's base URL otherwise. */
export function selfHostedBaseUrl(backendUrl: string | null | undefined): string | null {
	if (isCloudUrl(backendUrl)) return null;
	return normalizeBaseUrl(backendUrl) ?? (backendUrl?.trim() || null);
}
