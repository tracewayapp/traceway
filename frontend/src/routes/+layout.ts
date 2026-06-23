import { browser } from '$app/environment';

export const ssr = false;
export const prerender = true;

export const load = async ({ url }) => {
	if (browser) {
		const projectId = url.searchParams.get('projectId');
		if (projectId) {
			const { projectsState } = await import('$lib/state/projects.svelte');
			projectsState.selectProject(projectId);
		}
	}
	return {};
};
