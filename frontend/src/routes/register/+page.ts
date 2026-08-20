import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = () => {
	const token = localStorage.getItem('AUTH_TOKEN');
	if (token) {
		const cached = JSON.parse(localStorage.getItem('PROJECTS_CACHE_V2') || '[]');
		throw redirect(302, cached.length === 0 ? '/setup' : '/');
	}
	return {};
};
