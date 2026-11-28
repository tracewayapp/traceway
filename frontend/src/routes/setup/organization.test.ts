import { cleanup, render, waitFor } from '@testing-library/svelte';
import { afterEach, expect, it, vi } from 'vitest';
import { authState } from '$lib/state/auth.svelte';
import { projectsState } from '$lib/state/projects.svelte';
import SetupPage from './+page.svelte';

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }));
vi.mock('$lib/api', () => ({ api: { get, post } }));

afterEach(() => {
	cleanup();
	authState.setOrganizations([]);
	vi.restoreAllMocks();
	vi.clearAllMocks();
});

it('mints for the requested organization after refresh and again for the next setup link', async () => {
	const org = (id: number) => ({ id, name: 'Organization ' + id, role: 'owner', timezone: 'UTC' });
	authState.setOrganizations([org(1)]);
	const bundle = Promise.withResolvers<{ organizations: ReturnType<typeof org>[]; projects: [] }>();
	get.mockImplementation((url: string) =>
		url === '/me/login-bundle' ? bundle.promise : Promise.resolve({ draft: null })
	);
	post.mockResolvedValue(null);
	vi.spyOn(projectsState, 'loadProjects').mockResolvedValue(undefined);
	const { rerender } = render(SetupPage, { data: { organizationId: 2, existingAccount: true } });
	await waitFor(() => expect(get).toHaveBeenCalledWith('/me/login-bundle'));
	expect(get).not.toHaveBeenCalledWith('/setup/drafts?organizationId=1');
	bundle.resolve({ organizations: [org(1), org(2)], projects: [] });
	await waitFor(() => expect(post).toHaveBeenCalledWith('/setup-tokens', { organizationId: 2 }));
	await rerender({ data: { organizationId: 1, existingAccount: true } });
	await waitFor(() => expect(post).toHaveBeenCalledWith('/setup-tokens', { organizationId: 1 }));
	expect(post.mock.calls).toEqual([
		['/setup-tokens', { organizationId: 2 }],
		['/setup-tokens', { organizationId: 1 }]
	]);
});
