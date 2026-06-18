import { redirect } from '@sveltejs/kit';
import { authState } from '$lib/state/auth.svelte';
import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = ({ params, url }) => {
	if (!authState.isAuthenticated) {
		throw redirect(302, '/login');
	}

	return {
		traceName: params.traceName,
		traceId: params.traceId,
		recordedAt: url.searchParams.get('t')
	};
};
