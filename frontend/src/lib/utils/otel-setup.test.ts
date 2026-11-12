import { describe, expect, it } from 'vitest';
import { OTEL_TARGETS, legacyOtelGuide } from './otel-setup';
import { LEGACY_GO_FRAMEWORKS, usesOtelSetupGuide } from '$lib/state/projects.svelte';

describe('legacy Go projects', () => {
	it('open the OpenTelemetry guide on Go and a framework the guide offers', () => {
		const go = OTEL_TARGETS.find((t) => t.id === 'go');
		for (const framework of LEGACY_GO_FRAMEWORKS) {
			expect(usesOtelSetupGuide(framework)).toBe(true);
			const guide = legacyOtelGuide(framework);
			expect(guide?.target).toBe('go');
			if (guide?.framework) {
				expect(go?.frameworks.some((f) => f.id === guide.framework)).toBe(true);
			}
		}
		expect(legacyOtelGuide('gin')).toEqual({ target: 'go', framework: 'gin' });
		expect(legacyOtelGuide('stdlib')).toEqual({ target: 'go', framework: 'nethttp' });
	});

	it('leave every other project on its own setup', () => {
		expect(usesOtelSetupGuide('opentelemetry')).toBe(true);
		expect(legacyOtelGuide('opentelemetry')).toBeNull();
		for (const framework of [
			'react',
			'nestjs',
			'django',
			'hono',
			'cloudflare',
			'flutter'
		] as const) {
			expect(usesOtelSetupGuide(framework)).toBe(false);
			expect(legacyOtelGuide(framework)).toBeNull();
		}
	});
});
