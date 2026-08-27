import { describe, expect, it } from 'vitest';

import { resolveHref } from './links';

describe('resolveHref', () => {
	// resolve() from $app/paths reads these as route ids and dereferences a
	// params argument resolveHref never had, throwing a TypeError. Any returnTo=
	// reaches this, so a login could land the user logged in but stranded on the
	// login page reading the TypeError as a credentials error.
	it('passes bracketed paths through instead of throwing', () => {
		expect(resolveHref('/a[b]c')).toBe('/a[b]c');
		expect(resolveHref('/tasks/[task]')).toBe('/tasks/[task]');
		expect(resolveHref('/issues/[hash]/events?preset=24h')).toBe(
			'/issues/[hash]/events?preset=24h'
		);
	});

	it('keeps ordinary app paths and their suffixes intact', () => {
		expect(resolveHref('/issues/abc123')).toBe('/issues/abc123');
		expect(resolveHref('/issues?projectId=one')).toBe('/issues?projectId=one');
		expect(resolveHref('/monitors#incidents')).toBe('/monitors#incidents');
		expect(resolveHref('')).toBe('/');
	});

	it('leaves anything that is not an app path alone', () => {
		expect(resolveHref('https://example.com/a')).toBe('https://example.com/a');
		expect(resolveHref('//example.com/a')).toBe('//example.com/a');
		expect(resolveHref('#section')).toBe('#section');
		// button.svelte and otel-setup-steps.svelte forward hrefs they do not
		// control; a scheme-only URI must not have the base path glued onto it.
		expect(resolveHref('mailto:support@example.com')).toBe('mailto:support@example.com');
		expect(resolveHref('tel:+15551234')).toBe('tel:+15551234');
	});
});
