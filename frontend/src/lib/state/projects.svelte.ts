import { api } from '$lib/api';
import { authState } from './auth.svelte';

export type Framework =
	| 'gin'
	| 'fiber'
	| 'chi'
	| 'fasthttp'
	| 'stdlib'
	| 'drift'
	| 'custom'
	| 'react'
	| 'svelte'
	| 'vuejs'
	| 'nextjs'
	| 'nestjs'
	| 'express'
	| 'remix';

export const FRONTEND_FRAMEWORKS: Framework[] = ['drift', 'react', 'svelte', 'vuejs'];
export const JS_FRAMEWORKS: Framework[] = [
	'react',
	'svelte',
	'vuejs',
	'nextjs',
	'nestjs',
	'express',
	'remix'
];

export const FRAMEWORK_LABELS: Record<Framework, string> = {
	gin: 'Gin',
	fiber: 'Fiber',
	chi: 'Chi',
	fasthttp: 'FastHTTP',
	stdlib: 'Standard Library',
	drift: 'Drift',
	custom: 'Custom',
	react: 'React',
	svelte: 'Svelte',
	vuejs: 'Vue.js',
	nextjs: 'Next.js',
	nestjs: 'NestJS',
	express: 'Express',
	remix: 'Remix'
};

export function getFrameworkLabel(fw: Framework): string {
	return FRAMEWORK_LABELS[fw] ?? fw;
}

export function isFrontendFramework(fw: Framework): boolean {
	return FRONTEND_FRAMEWORKS.includes(fw);
}

export function isJsFramework(fw: Framework): boolean {
	return JS_FRAMEWORKS.includes(fw);
}

export interface Project {
	id: string;
	name: string;
	token: string;
	framework: Framework;
	organizationId: number | null;
	createdAt: string;
	backendUrl: string;
}

export interface ProjectWithToken extends Project {
	token: string;
}

class ProjectsState {
	projects = $state<Project[]>(JSON.parse(localStorage.getItem('PROJECTS_CACHE') || '[]'));
	currentProjectId = $state<string | null>(localStorage.getItem('CURRENT_PROJECT_ID'));
	loading = $state(false);
	error = $state<string | null>(null);

	currentProject = $derived(
		this.projects.find((p) => p.id === this.currentProjectId) || this.projects[0] || null
	);

	canManageCurrentProject = $derived.by(() => {
		const organizationId = this.currentProject?.organizationId;
		if (!organizationId) return false;
		return authState.canManageOrganization(organizationId);
	});

	constructor() {
		$effect.root(() => {
			$effect(() => {
				if (this.currentProjectId) {
					localStorage.setItem('CURRENT_PROJECT_ID', this.currentProjectId);
				} else {
					localStorage.removeItem('CURRENT_PROJECT_ID');
				}
			});
		});
	}

	setProjects(projects: Project[]) {
		this.projects = projects;

		// If no current project selected or current project not in list, select first one
		if (!this.currentProjectId || !this.projects.find((p) => p.id === this.currentProjectId)) {
			if (this.projects.length > 0) {
				this.currentProjectId = this.projects[0].id;
			}
		}

		// Cache in localStorage
		localStorage.setItem('PROJECTS_CACHE', JSON.stringify(this.projects));
	}

	async loadProjects() {
		this.loading = true;
		this.error = null;

		try {
			const response = await api.get('/projects');
			this.setProjects(response);
		} catch (e: unknown) {
			const errorMessage = e instanceof Error ? e.message : 'Failed to load projects';
			this.error = errorMessage;

			// Try to load from cache
			const cached = localStorage.getItem('PROJECTS_CACHE');
			if (cached) {
				this.projects = JSON.parse(cached);
			}
		} finally {
			this.loading = false;
		}
	}

	async createProject(name: string, framework: Framework = 'gin'): Promise<ProjectWithToken> {
		const response = await api.post('/projects', { name, framework });

		// Reload projects to refresh cache
		await this.loadProjects();

		return response;
	}

	selectProject(projectId: string) {
		this.currentProjectId = projectId;
	}

	initFromCache() {
		const cached = localStorage.getItem('PROJECTS_CACHE');
		if (cached) {
			this.projects = JSON.parse(cached);
		}
	}
}

export const projectsState = new ProjectsState();
