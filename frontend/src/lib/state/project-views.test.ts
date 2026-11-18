import { describe, expect, it } from 'vitest';
import { isProjectViewAvailable, SDK_CONFIG_FRAMEWORKS } from './projects.svelte';

describe('project view availability', () => {
	it.each(['react-native', 'android', 'ios'] as const)(
		'keeps browser collection and full session views off %s',
		(framework) => {
			expect(SDK_CONFIG_FRAMEWORKS).not.toContain(framework);
			for (const path of [
				'/sessions',
				'/sessions/session-id',
				'/data-collection',
				'/slos',
				'/slos/slo-id',
				'/dashboards'
			]) {
				expect(isProjectViewAvailable(framework, path), path).toBe(false);
			}
			for (const path of ['/issues', '/issues/hash/event', '/connection', '/notifications']) {
				expect(isProjectViewAvailable(framework, path), path).toBe(true);
			}
		}
	);
	it.each(['react', 'vuejs', 'svelte', 'jquery'] as const)(
		'enables browser views for %s',
		(framework) => {
			expect(SDK_CONFIG_FRAMEWORKS).toContain(framework);
			for (const path of [
				'/sessions',
				'/sessions/id',
				'/data-collection',
				'/slos',
				'/dashboards'
			]) {
				expect(isProjectViewAvailable(framework, path), path).toBe(true);
			}
		}
	);
	it('enables the session, collection, dashboard and SLO views for flutter', () => {
		expect(SDK_CONFIG_FRAMEWORKS).toContain('flutter');
		for (const path of [
			'/sessions',
			'/sessions/session-id',
			'/data-collection',
			'/slos',
			'/slos/slo-id',
			'/dashboards',
			'/logs',
			'/spans',
			'/issues',
			'/connection'
		]) {
			expect(isProjectViewAvailable('flutter', path), path).toBe(true);
		}
	});
	it.each(['opentelemetry', 'gin', 'nestjs', 'nextjs', 'remix'] as const)(
		'preserves backend monitoring for %s',
		(framework) => {
			expect(isProjectViewAvailable(framework, '/slos')).toBe(true);
			expect(isProjectViewAvailable(framework, '/dashboards')).toBe(true);
			expect(isProjectViewAvailable(framework, '/sessions')).toBe(false);
			expect(isProjectViewAvailable(framework, '/data-collection')).toBe(false);
		}
	);
});
