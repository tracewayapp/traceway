import { page } from '$app/state';
import { api } from '$lib/api';
import { authState } from './auth.svelte';
import { isBackendFramework, projectsState, type Project } from './projects.svelte';

export function isOrganizationPath(pathname: string): boolean {
	return pathname === '/organization' || pathname.startsWith('/organization/');
}

export function organizationHref(path: string, organizationId: number): string {
	return `${path}?organizationId=${organizationId}`;
}

class OrganizationContext {
	active = $derived(isOrganizationPath(page.url.pathname));

	organizationId = $derived.by(() => {
		if (!this.active) return null;
		const value = Number(page.url.searchParams.get('organizationId'));
		if (
			Number.isInteger(value) &&
			authState.organizations.some((organization) => organization.id === value)
		) {
			return value;
		}
		return authState.organizations[0]?.id ?? null;
	});

	organization = $derived(
		authState.organizations.find((organization) => organization.id === this.organizationId) ?? null
	);

	projects = $derived<Project[]>(
		this.organizationId === null
			? []
			: projectsState.projects.filter((project) => project.organizationId === this.organizationId)
	);

	hasBackendProjects = $derived(
		this.projects.some((project) => isBackendFramework(project.framework))
	);

	canManage = $derived(
		this.organizationId !== null && authState.canManageOrganization(this.organizationId)
	);

	openPagesCount = $state(0);
	downMonitorsCount = $state(0);

	async refreshCounts(organizationId: number) {
		try {
			const response = await api.get(`/organizations/${organizationId}/overview/counts`);
			if (this.organizationId !== organizationId) return;
			this.openPagesCount = response.openPagesCount ?? 0;
			this.downMonitorsCount = response.downMonitorsCount ?? 0;
		} catch {
			return;
		}
	}
}

export const organizationContext = new OrganizationContext();
