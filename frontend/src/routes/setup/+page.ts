import { redirect } from '@sveltejs/kit';
import { authState } from '$lib/state/auth.svelte';
import { findPendingProposalOrganization, setupEntry } from './entry';
import type { PageLoad } from './$types';

function cachedProjectCount(): number {
	try {
		const cached = JSON.parse(localStorage.getItem('PROJECTS_CACHE_V2') || '[]');
		return Array.isArray(cached) ? cached.length : 0;
	} catch {
		return 0;
	}
}

// The setup flow is the onboarding of zero-project accounts. An account with projects comes here to mint a setup
// token or to decide a proposal: through ?mode=add or ?organizationId=<id> (the links the CLI and the skill give),
// or while one of its organizations has a proposal waiting. Any other visit goes to the dashboard, where new
// projects are added from the project dropdown.
export const load: PageLoad = async ({ url }) => {
	if (!authState.isAuthenticated) {
		throw redirect(302, `/login?returnTo=${encodeURIComponent(url.pathname + url.search)}`);
	}
	const entry = setupEntry(url.searchParams, cachedProjectCount());
	if (entry) return entry;

	const writable = authState.organizations.filter((o) => o.role !== 'readonly').map((o) => o.id);
	const pending = await findPendingProposalOrganization(writable);
	if (pending !== null) return { existingAccount: true, organizationId: pending };
	throw redirect(302, '/');
};
