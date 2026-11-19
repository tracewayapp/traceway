/**
 * Keeps an element out of the dashboard's own session replay. The browser SDK hands rrweb nothing but
 * `maskAllInputs`, which remote config can switch off, so rrweb's default block class is the one mechanism
 * that always applies: a blocked element is recorded as an empty box of the same size, without its text,
 * attributes, input values or children. Put it on every element that shows a credential.
 */
export const REPLAY_BLOCK_CLASS = 'rr-block';

const UNMONITORED_PREFIXES = ['/status/', '/ack/'];
const UNMONITORED_PATHS = new Set([
	'/status',
	'/reset-password',
	'/accept-invitation',
	'/auth/callback'
]);
const CREDENTIAL_IN_URL = /(^|[?&#])token=|tw[kpw]_/i;

function carriesCredential(url: URL): boolean {
	let text = url.search + url.hash;
	// A returnTo parameter nests another URL, encoded once more per level.
	for (let depth = 0; depth < 4; depth++) {
		if (CREDENTIAL_IN_URL.test(text)) return true;
		let decoded: string;
		try {
			decoded = decodeURIComponent(text);
		} catch {
			return false;
		}
		if (decoded === text) return false;
		text = decoded;
	}
	return CREDENTIAL_IN_URL.test(text);
}

/**
 * Whether the dashboard may start reporting itself (errors, network spans, navigation and session replay) on
 * this URL. Public status pages and custom status domains are visited by the public, not by dashboard users,
 * and the pages that carry a credential in their URL (password reset, invitation, page acknowledgement, the
 * SSO callback) would put it into the recorded URLs.
 */
export function shouldMonitorUrl(url: URL, statusDomainSlug: string | null): boolean {
	if (statusDomainSlug !== null) return false;
	const path = url.pathname.replace(/\/+$/, '') || '/';
	if (UNMONITORED_PATHS.has(path) || UNMONITORED_PREFIXES.some((prefix) => path.startsWith(prefix)))
		return false;
	return !carriesCredential(url);
}
