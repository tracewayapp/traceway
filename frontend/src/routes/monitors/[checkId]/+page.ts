import { redirect } from '@sveltejs/kit';
import { authState } from '$lib/state/auth.svelte';
import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = ({ params, url }) => {
	// Old sub-page URLs; without this they'd match [checkId]. Status pages are
	// now a tab; the runners page was removed (runners are operator infra).
	if (params.checkId === 'status-pages') {
		throw redirect(301, '/monitors?tab=status-pages');
	}
	if (params.checkId === 'runners') {
		throw redirect(301, '/monitors');
	}
	if (!authState.isAuthenticated) throw redirect(302, '/login');
	return {
		checkId: params.checkId,
		preset: url.searchParams.get('preset'),
		from: url.searchParams.get('from'),
		to: url.searchParams.get('to')
	};
};
