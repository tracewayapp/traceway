import { describe, expect, it } from 'vitest';
import { shouldMonitorUrl } from './self-monitoring';

const at = (path: string) => new URL(path, 'https://traceway.example.com');

describe('shouldMonitorUrl', () => {
	it('monitors dashboard pages', () => {
		for (const path of [
			'/',
			'/issues?preset=24h',
			'/account?tab=tokens',
			'/login',
			'/login?returnTo=%2Fissues%3Fpreset%3D7d',
			'/settings?tab=agent-access',
			'/statuses'
		])
			expect(shouldMonitorUrl(at(path), null), path).toBe(true);
	});

	it('never monitors public status pages or a custom status domain', () => {
		expect(shouldMonitorUrl(at('/status/acme'), null)).toBe(false);
		expect(shouldMonitorUrl(at('/status/acme/'), null)).toBe(false);
		expect(shouldMonitorUrl(at('/'), 'acme')).toBe(false);
		expect(shouldMonitorUrl(at('/issues'), 'acme')).toBe(false);
	});

	it('does not monitor pages that carry a credential in their URL', () => {
		for (const path of [
			'/ack/twk_abc123',
			'/reset-password?token=abc',
			'/accept-invitation?token=abc',
			'/auth/callback#token=eyJhbGciOi&needsSetup=false',
			'/login?returnTo=%2Faccept-invitation%3Ftoken%3Dabc',
			'/login?returnTo=' + encodeURIComponent('/login?returnTo=/accept-invitation?token=abc'),
			'/issues?search=twp_abc123'
		])
			expect(shouldMonitorUrl(at(path), null), path).toBe(false);
	});

	it('tolerates malformed escapes', () => {
		expect(shouldMonitorUrl(at('/logs?search=100%25'), null)).toBe(true);
	});
});
