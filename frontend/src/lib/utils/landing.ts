export interface LandingOrganization {
	id: number;
}

export interface LandingProject {
	id: string;
	organizationId: number | null;
}

export function safeLocalPath(raw: string | null): string {
	if (!raw) return '/';
	let decoded: string;
	try {
		decoded = decodeURIComponent(raw);
	} catch {
		return '/';
	}
	if (!decoded.startsWith('/') || decoded.startsWith('//') || decoded.startsWith('/\\')) {
		return '/';
	}
	return decoded;
}

export function defaultAuthenticatedPath(
	organizations: LandingOrganization[],
	projects: LandingProject[]
): string {
	const firstOrganization = organizations.find((organization) =>
		projects.some((project) => project.organizationId === organization.id)
	);
	if (firstOrganization) {
		const organizationProjects = projects.filter(
			(project) => project.organizationId === firstOrganization.id
		);
		if (organizationProjects.length > 1) {
			return `/organization?organizationId=${firstOrganization.id}`;
		}
		return `/?projectId=${organizationProjects[0].id}`;
	}

	return projects[0] ? `/?projectId=${projects[0].id}` : '/setup';
}

export function authenticatedLandingPath(
	returnTo: string | null,
	organizations: LandingOrganization[],
	projects: LandingProject[]
): string {
	const requestedPath = safeLocalPath(returnTo);
	return requestedPath === '/' ? defaultAuthenticatedPath(organizations, projects) : requestedPath;
}
