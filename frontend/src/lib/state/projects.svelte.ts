import { api } from '$lib/api';
import { authState } from './auth.svelte';

export type Framework =
	| 'gin'
	| 'fiber'
	| 'chi'
	| 'fasthttp'
	| 'stdlib'
	| 'custom'
	| 'react'
	| 'svelte'
	| 'vuejs'
	| 'nextjs'
	| 'nestjs'
	| 'express'
	| 'remix'
	| 'jquery'
	| 'react-native'
	| 'hono'
	| 'cloudflare'
	| 'opentelemetry'
	| 'symfony'
	| 'laravel'
	| 'django'
	| 'flutter'
	| 'android'
	| 'ios';

export const FRONTEND_FRAMEWORKS: Framework[] = [
	'react',
	'svelte',
	'vuejs',
	'jquery',
	'react-native',
	'flutter',
	'android',
	'ios'
];
export const JS_FRAMEWORKS: Framework[] = [
	'react',
	'svelte',
	'vuejs',
	'nextjs',
	'nestjs',
	'express',
	'remix',
	'jquery',
	'react-native'
];

export const FRAMEWORK_LABELS: Record<Framework, string> = {
	gin: 'Gin',
	fiber: 'Fiber',
	chi: 'Chi',
	fasthttp: 'FastHTTP',
	stdlib: 'Standard Library',
	custom: 'Custom',
	react: 'React',
	svelte: 'Svelte',
	vuejs: 'Vue.js',
	nextjs: 'Next.js',
	nestjs: 'NestJS',
	express: 'Express',
	remix: 'Remix',
	jquery: 'jQuery',
	'react-native': 'React Native',
	hono: 'Hono',
	cloudflare: 'Cloudflare',
	opentelemetry: 'OpenTelemetry',
	symfony: 'Symfony',
	laravel: 'Laravel',
	django: 'Django',
	flutter: 'Flutter',
	android: 'Android',
	ios: 'iOS'
};

export const MOBILE_FRAMEWORKS: Framework[] = ['flutter', 'android', 'ios'];

export function getFrameworkLabel(fw: Framework): string {
	return FRAMEWORK_LABELS[fw] ?? fw;
}

export function isMobileFramework(fw: Framework): boolean {
	return MOBILE_FRAMEWORKS.includes(fw);
}

export function isFrontendFramework(fw: Framework): boolean {
	return FRONTEND_FRAMEWORKS.includes(fw);
}

export function isJsFramework(fw: Framework): boolean {
	return JS_FRAMEWORKS.includes(fw);
}

export function supportsSymbolUpload(fw: Framework): boolean {
	return isJsFramework(fw) || fw === 'flutter' || fw === 'ios' || fw === 'android';
}

export const JS_LANGUAGES = ['webjs', 'nodejs', 'javascript', 'typescript'];

export function isJsLanguage(lang?: string | null): boolean {
	return !!lang && JS_LANGUAGES.includes(lang.toLowerCase());
}

export function isOtelFramework(fw: Framework): boolean {
	return fw === 'opentelemetry' || fw === 'hono';
}

export function isCloudflareFramework(fw: Framework): boolean {
	return fw === 'cloudflare';
}

export interface Project {
	id: string;
	name: string;
	token: string;
	framework: Framework;
	organizationId: number | null;
	createdAt: string;
	sourceMapToken: string | null;
	backendUrl: string;
	dropHealthyHealthchecks: boolean;
	healthcheckPaths: string[] | null;
	profileLabelAllowlist: string[] | null;
	aiFlaggedTerms: string[] | null;
	aiFlaggedLanguages: string[] | null;
	role?: string;
}

export function isProjectReadonly(project: Project | null): boolean {
	if (!project) return false;
	if (project.role) return project.role === 'readonly';
	if (!project.organizationId) return false;
	return authState.getRoleForOrganization(project.organizationId) === 'readonly';
}

export interface ProjectWithToken extends Project {
	token: string;
}

const PROJECTS_CACHE_KEY = 'PROJECTS_CACHE_V2';

class ProjectsState {
	projects = $state<Project[]>(JSON.parse(localStorage.getItem(PROJECTS_CACHE_KEY) || '[]'));
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

	canWriteCurrentProject = $derived(!isProjectReadonly(this.currentProject));

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
			this.currentProjectId = this.projects.length > 0 ? this.projects[0].id : null;
		}

		// Cache in localStorage
		localStorage.setItem(PROJECTS_CACHE_KEY, JSON.stringify(this.projects));
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
			const cached = localStorage.getItem(PROJECTS_CACHE_KEY);
			if (cached) {
				this.projects = JSON.parse(cached);
			}
		} finally {
			this.loading = false;
		}
	}

	// Batch create through POST /projects/batch, which needs no existing
	// projectId, so it also works for zero-project accounts (the registration
	// wizard's manual step). Existing names are returned instead of erroring.
	async createProjects(
		organizationId: number,
		projects: { name: string; framework: Framework }[]
	): Promise<ProjectWithToken[]> {
		const response = await api.post('/projects/batch', { organizationId, projects });
		await this.loadProjects();
		return response.projects;
	}

	async createProject(
		name: string,
		framework: Framework = 'gin',
		organizationId?: number
	): Promise<ProjectWithToken> {
		const targetOrgId = organizationId ?? this.currentProject?.organizationId;
		if (!targetOrgId) {
			throw new Error('No organization selected');
		}
		const created = await this.createProjects(targetOrgId, [{ name, framework }]);
		return created[0];
	}

	async updateProject(
		id: string,
		name: string,
		framework: Framework,
		dropHealthyHealthchecks: boolean,
		healthcheckPaths: string[],
		profileLabelAllowlist: string[],
		aiFlaggedTerms: string[],
		aiFlaggedLanguages: string[]
	): Promise<Project> {
		const response = await api.put(
			'/projects',
			{
				name,
				framework,
				dropHealthyHealthchecks,
				healthcheckPaths,
				profileLabelAllowlist,
				aiFlaggedTerms,
				aiFlaggedLanguages
			},
			{
				projectId: id
			}
		);
		await this.loadProjects();
		return response;
	}

	async deleteProject(id: string, name: string): Promise<void> {
		await api.delete('/projects', { projectId: id }, { name });
		await this.loadProjects();
	}

	async generateSourceMapToken(): Promise<string> {
		const resp = await api.post(
			'/projects/source-map-token',
			{},
			{
				projectId: this.currentProjectId ?? undefined
			}
		);
		await this.loadProjects();
		return resp.sourceMapToken;
	}

	selectProject(projectId: string) {
		this.currentProjectId = projectId;
	}

	clear() {
		this.projects = [];
		this.currentProjectId = null;
		localStorage.removeItem(PROJECTS_CACHE_KEY);
	}

	initFromCache() {
		const cached = localStorage.getItem(PROJECTS_CACHE_KEY);
		if (cached) {
			this.projects = JSON.parse(cached);
		}
	}
}

export const projectsState = new ProjectsState();
