import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const ssr = false;

function cachedArray(key: string): unknown[] {
	try {
		const value = JSON.parse(localStorage.getItem(key) || '[]');
		return Array.isArray(value) ? value : [];
	} catch {
		return [];
	}
}

export const load: PageLoad = () => {
	if (!localStorage.getItem('AUTH_TOKEN')) return {};

	const organizations = cachedArray('USER_ORGANIZATIONS');
	if (organizations.length === 0) return {};

	const projects = cachedArray('PROJECTS_CACHE_V2');
	throw redirect(302, projects.length === 0 ? '/setup' : '/');
};
