import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = () => {
	// An already-authenticated visitor doesn't belong on the register form.
	// Zero-project accounts (someone who left the wizard after step 1) land
	// on /setup, which is the same project step; everyone else goes home.
	const token = localStorage.getItem('AUTH_TOKEN');
	if (token) {
		const cached = JSON.parse(localStorage.getItem('PROJECTS_CACHE_V2') || '[]');
		throw redirect(302, cached.length === 0 ? '/setup' : '/');
	}
	return {};
};
