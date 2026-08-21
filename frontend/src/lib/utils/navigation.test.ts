import { describe, expect, it } from 'vitest';

import { authenticatedLandingPath, defaultAuthenticatedPath } from './landing';

describe('defaultAuthenticatedPath', () => {
	it('opens the first organization when it has multiple projects', () => {
		expect(
			defaultAuthenticatedPath(
				[{ id: 10 }, { id: 20 }],
				[
					{ id: 'other', organizationId: 20 },
					{ id: 'first-a', organizationId: 10 },
					{ id: 'first-b', organizationId: 10 }
				]
			)
		).toBe('/organization?organizationId=10');
	});

	it('opens the project directly when the first organization has one project', () => {
		expect(
			defaultAuthenticatedPath(
				[{ id: 10 }, { id: 20 }],
				[
					{ id: 'other', organizationId: 20 },
					{ id: 'only', organizationId: 10 }
				]
			)
		).toBe('/?projectId=only');
	});

	it('falls back to the first available project, then setup', () => {
		expect(defaultAuthenticatedPath([{ id: 10 }], [{ id: 'other', organizationId: 20 }])).toBe(
			'/?projectId=other'
		);
		expect(defaultAuthenticatedPath([], [])).toBe('/setup');
	});

	it('skips an empty organization when a later membership has projects', () => {
		expect(
			defaultAuthenticatedPath(
				[{ id: 10 }, { id: 20 }],
				[
					{ id: 'second-a', organizationId: 20 },
					{ id: 'second-b', organizationId: 20 }
				]
			)
		).toBe('/organization?organizationId=20');
	});
});

describe('authenticatedLandingPath', () => {
	it('uses the default destination for an absent or root return path', () => {
		const organizations = [{ id: 10 }];
		const projects = [
			{ id: 'one', organizationId: 10 },
			{ id: 'two', organizationId: 10 }
		];

		expect(authenticatedLandingPath(null, organizations, projects)).toBe(
			'/organization?organizationId=10'
		);
		expect(authenticatedLandingPath('/', organizations, projects)).toBe(
			'/organization?organizationId=10'
		);
	});

	it('preserves a safe explicit destination', () => {
		expect(authenticatedLandingPath('/issues?projectId=one', [{ id: 10 }], [])).toBe(
			'/issues?projectId=one'
		);
	});
});
