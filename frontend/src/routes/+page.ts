import { redirect } from '@sveltejs/kit';
import { authState } from '$lib/state/auth.svelte';
import { resolveStatusDomain } from '$lib/utils/status-domain';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const statusSlug = await resolveStatusDomain(fetch);
	if (statusSlug) {
		return { statusSlug };
	}
	if (!authState.isAuthenticated) {
		throw redirect(302, '/login');
	}
	return { statusSlug: null };
};
