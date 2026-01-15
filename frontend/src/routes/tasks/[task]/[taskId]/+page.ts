import { redirect } from '@sveltejs/kit';
import { authState } from '$lib/state/auth.svelte';
import type { PageLoad } from './$types';

export const prerender = false;

export const load: PageLoad = ({ params, url }) => {
	if (!authState.isAuthenticated) {
		throw redirect(302, '/login');
	}

	return {
		task: params.task,
		taskId: params.taskId,
		preset: url.searchParams.get('preset') || null,
		from: url.searchParams.get('from') || null,
		to: url.searchParams.get('to') || null
	};
};
