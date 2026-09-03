import { authState } from './state/auth.svelte';
import { projectsState } from './state/projects.svelte';
import { gotoHref } from './utils/navigation';
import { toast } from 'svelte-sonner';

const BASE_URL = '/api';

interface RequestOptions {
	projectId?: string;
	skipProjectId?: boolean;
}

let recovering = false;
let staleProjectNotified = '';

async function request(method: string, endpoint: string, data?: unknown, options?: RequestOptions) {
	const token = authState.token;
	const headers: Record<string, string> = { 'Content-Type': 'application/json' };
	if (token) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	let url = `${BASE_URL}${endpoint}`;
	if (options?.projectId && !options?.skipProjectId) {
		url += `${endpoint.includes('?') ? '&' : '?'}projectId=${options.projectId}`;
	}

	const response = await fetch(url, {
		method,
		headers,
		body: data ? JSON.stringify(data) : undefined
	});

	if (authState.token !== token) {
		throw Object.assign(new Error('Session changed'), { status: 0 });
	}

	if (response.status === 401) {
		authState.logout();
		const current = window.location.pathname + window.location.search;
		gotoHref(
			current === '/' || current.startsWith('/login')
				? '/login'
				: `/login?returnTo=${encodeURIComponent(current)}`,
			{ replaceState: true }
		);
		throw Object.assign(new Error('Unauthorized'), { status: 401 });
	}

	if (response.status === 403) {
		const body = await response.json().catch(() => ({}));
		const message =
			body.error ||
			(method === 'GET' ? 'Forbidden' : "You don't have permission to perform this action");

		if (!recovering) {
			recovering = true;
			try {
				const bundle = await request('GET', '/me/login-bundle').catch(() => null);
				if (bundle) {
					authState.setOrganizations(bundle.organizations ?? []);
					projectsState.setProjects(bundle.projects ?? []);
				}

				if (authState.token === token) {
					const requestedProjectId = new URL(url, window.location.origin).searchParams.get(
						'projectId'
					);
					const staleProject =
						bundle &&
						requestedProjectId &&
						!projectsState.projects.some((project) => project.id === requestedProjectId);

					if (staleProject) {
						if (staleProjectNotified !== requestedProjectId) {
							staleProjectNotified = requestedProjectId;
							toast.warning('That project is no longer available');
						}
						const current = new URL(window.location.href);
						if (
							current.searchParams.get('projectId') === requestedProjectId &&
							projectsState.currentProjectId
						) {
							current.searchParams.set('projectId', projectsState.currentProjectId);
							await gotoHref(current.pathname + current.search, { replaceState: true });
						}
					} else if (method !== 'GET') {
						toast.error(message);
					} else {
						toast.warning("You don't have permission to access that feature");
						if (window.location.pathname !== '/') {
							await gotoHref('/', { replaceState: true });
						}
					}
				}
			} finally {
				recovering = false;
			}
		}

		throw Object.assign(new Error(message), { status: 403, body });
	}

	if (response.status === 422) {
		const body = await response.json().catch(() => ({}));
		throw Object.assign(new Error(body.error || 'Validation failed'), { status: 422, body });
	}

	if (!response.ok) {
		throw Object.assign(new Error(`API Error: ${response.statusText}`), {
			status: response.status
		});
	}

	return response.json();
}

export const api = {
	get: (endpoint: string, options?: RequestOptions) => request('GET', endpoint, undefined, options),
	post: (endpoint: string, data: unknown, options?: RequestOptions) =>
		request('POST', endpoint, data, options),
	put: (endpoint: string, data: unknown, options?: RequestOptions) =>
		request('PUT', endpoint, data, options),
	delete: (endpoint: string, options?: RequestOptions, data?: unknown) =>
		request('DELETE', endpoint, data, options)
};
