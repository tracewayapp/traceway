import { redirect } from '@sveltejs/kit';
import { authState } from '$lib/state/auth.svelte';
import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = () => {
	const canManage = authState.organizations.some((o) => o.role === 'owner' || o.role === 'admin');
	if (!canManage) {
		throw redirect(302, '/');
	}
	return {};
};
