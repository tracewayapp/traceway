import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const { get, auth } = vi.hoisted(() => ({
	get: vi.fn(),
	auth: {
		isAuthenticated: true,
		organizations: [] as { id: number; name: string; role: string; timezone: string }[]
	}
}));
vi.mock('$lib/api', () => ({ api: { get } }));
vi.mock('$lib/state/auth.svelte', () => ({ authState: auth }));

import { findPendingProposalOrganization, setupEntry, setupOrganization } from './entry';
import { load } from './+page';

type LoadEvent = Parameters<typeof load>[0];

async function runLoad(path: string) {
	return load({ url: new URL(path, 'http://localhost') } as LoadEvent);
}

async function redirectOf(path: string): Promise<string> {
	try {
		await runLoad(path);
	} catch (e) {
		const redirect = e as { status?: number; location?: string };
		if (redirect.status === 302 && redirect.location) return redirect.location;
		throw e;
	}
	throw new Error(`${path} did not redirect`);
}

function draftResponse(status: string | null) {
	return status === null ? { draft: null } : { draft: { id: 'd1', status } };
}

beforeEach(() => {
	auth.isAuthenticated = true;
	auth.organizations = [
		{ id: 1, name: 'Default', role: 'owner', timezone: 'UTC' },
		{ id: 2, name: 'Read Only', role: 'readonly', timezone: 'UTC' },
		{ id: 3, name: 'Second', role: 'user', timezone: 'UTC' }
	];
	localStorage.setItem('PROJECTS_CACHE_V2', JSON.stringify([{ id: 'p1', organizationId: 1 }]));
});

afterEach(() => {
	localStorage.clear();
	get.mockReset();
});

describe('setupEntry', () => {
	it('always opens the onboarding flow for an account without projects', () => {
		expect(setupEntry(new URLSearchParams(''), 0)).toEqual({
			existingAccount: false,
			organizationId: null
		});
		expect(setupEntry(new URLSearchParams('organizationId=4'), 0)).toEqual({
			existingAccount: false,
			organizationId: 4
		});
	});

	it('opens for an account with projects when the link asks for it', () => {
		expect(setupEntry(new URLSearchParams('mode=add'), 3)).toEqual({
			existingAccount: true,
			organizationId: null
		});
		expect(setupEntry(new URLSearchParams('mode=add&organizationId=7'), 3)).toEqual({
			existingAccount: true,
			organizationId: 7
		});
		expect(setupEntry(new URLSearchParams('organizationId=7'), 3)).toEqual({
			existingAccount: true,
			organizationId: 7
		});
	});

	it('leaves a plain visit and unusable organization ids to the pending-proposal check', () => {
		expect(setupEntry(new URLSearchParams(''), 3)).toBeNull();
		expect(setupEntry(new URLSearchParams('mode=manual'), 3)).toBeNull();
		expect(setupEntry(new URLSearchParams('organizationId=abc'), 3)).toBeNull();
		expect(setupEntry(new URLSearchParams('organizationId=0'), 3)).toBeNull();
		expect(setupEntry(new URLSearchParams('organizationId=-2'), 3)).toBeNull();
	});
});

describe('findPendingProposalOrganization', () => {
	it('returns the first organization whose proposal waits for a decision', async () => {
		get.mockImplementation(async (path: string) =>
			draftResponse(path.endsWith('=1') ? 'approved' : 'pending')
		);
		await expect(findPendingProposalOrganization([1, 3])).resolves.toBe(3);
		expect(get).toHaveBeenCalledWith('/setup/drafts?organizationId=1');
		expect(get).toHaveBeenCalledWith('/setup/drafts?organizationId=3');
	});

	it('returns null when no proposal is pending or the lookups fail', async () => {
		get.mockResolvedValueOnce(draftResponse(null)).mockRejectedValueOnce(new Error('boom'));
		await expect(findPendingProposalOrganization([1, 3])).resolves.toBeNull();
		await expect(findPendingProposalOrganization([])).resolves.toBeNull();
	});
});

describe('/setup load', () => {
	it('sends a logged-out visitor to login and back to the same link', async () => {
		auth.isAuthenticated = false;
		const location = await redirectOf('/setup?mode=add&organizationId=1');
		expect(location).toBe('/login?returnTo=%2Fsetup%3Fmode%3Dadd%26organizationId%3D1');
		expect(new URL(location, 'http://localhost').searchParams.get('returnTo')).toBe(
			'/setup?mode=add&organizationId=1'
		);
	});

	it('keeps the first-run flow for an account without projects', async () => {
		localStorage.setItem('PROJECTS_CACHE_V2', '[]');
		await expect(runLoad('/setup')).resolves.toEqual({
			existingAccount: false,
			organizationId: null
		});
		expect(get).not.toHaveBeenCalled();
	});

	it('opens for an account with projects on the CLI and skill links', async () => {
		await expect(runLoad('/setup?mode=add&organizationId=3')).resolves.toEqual({
			existingAccount: true,
			organizationId: 3
		});
		await expect(runLoad('/setup?mode=add')).resolves.toEqual({
			existingAccount: true,
			organizationId: null
		});
		expect(get).not.toHaveBeenCalled();
	});

	it('opens a plain /setup on the organization with a pending proposal', async () => {
		get.mockImplementation(async (path: string) =>
			draftResponse(path.endsWith('=3') ? 'pending' : null)
		);
		await expect(runLoad('/setup')).resolves.toEqual({ existingAccount: true, organizationId: 3 });
		expect(get).not.toHaveBeenCalledWith('/setup/drafts?organizationId=2');
	});

	it('sends a plain /setup to the dashboard when nothing is pending', async () => {
		get.mockResolvedValue(draftResponse('approved'));
		await expect(redirectOf('/setup')).resolves.toBe('/');
	});
});

describe('setup organization selection', () => {
	it('follows a new deep link and waits for the requested organization to appear on refresh', () => {
		const old = [{ id: 1 }];
		expect(setupOrganization(old, 1, 1, null)).toBe(1);
		expect(setupOrganization(old, 2, 1, null)).toBeNull();
		expect(setupOrganization([...old, { id: 2 }], 2, 1, null)).toBe(2);
	});
	it('honours an explicit picker choice, including for an unavailable destination', () => {
		expect(setupOrganization([{ id: 1 }], 2, null, 1)).toBe(1);
		expect(setupOrganization([{ id: 1 }], null, null, 2)).toBe(1);
	});
});
