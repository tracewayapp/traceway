import { redirect } from '@sveltejs/kit';
import { authState } from '$lib/state/auth.svelte';
import type { PageLoad } from './$types';

export const load: PageLoad = () => {
	if (!authState.isAuthenticated) {
		throw redirect(302, '/login?returnTo=%2Fsetup');
	}
	// The setup flow only exists for zero-project accounts; once projects
	// exist, new ones are added from the project dropdown instead.
	let cached: unknown = [];
	try {
		cached = JSON.parse(localStorage.getItem('PROJECTS_CACHE_V2') || '[]');
	} catch {
		cached = [];
	}
	if (Array.isArray(cached) && cached.length > 0) {
		throw redirect(302, '/');
	}
};
