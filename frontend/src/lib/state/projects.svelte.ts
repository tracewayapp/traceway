import { api } from '$lib/api';
import { authState } from './auth.svelte';

export type Framework = 'gin' | 'fiber' | 'chi' | 'fasthttp' | 'stdlib' | 'custom' | 'react' | 'svelte' | 'vuejs' | 'nextjs' | 'nestjs' | 'express' | 'remix' | 'jquery' | 'react-native' | 'hono' | 'cloudflare' | 'opentelemetry' | 'symfony' | 'laravel' | 'django' | 'flutter' | 'android';

export const FRONTEND_FRAMEWORKS: Framework[] = ['react', 'svelte', 'vuejs', 'jquery', 'react-native', 'flutter', 'android'];
export const JS_FRAMEWORKS: Framework[] = ['react', 'svelte', 'vuejs', 'nextjs', 'nestjs', 'express', 'remix', 'jquery', 'react-native'];

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
};

export const MOBILE_FRAMEWORKS: Framework[] = ['flutter', 'android'];

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
}

export interface ProjectWithToken extends Project {
    token: string;
}

class ProjectsState {
    projects = $state<Project[]>(
        JSON.parse(localStorage.getItem('PROJECTS_CACHE') || '[]')
    );
    currentProjectId = $state<string | null>(localStorage.getItem('CURRENT_PROJECT_ID'));
    loading = $state(false);
    error = $state<string | null>(null);

    currentProject = $derived(
        this.projects.find(p => p.id === this.currentProjectId) || this.projects[0] || null
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
        if (!this.currentProjectId || !this.projects.find(p => p.id === this.currentProjectId)) {
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
            this.setProjects(response)
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
        const response = await api.post('/projects', { name, framework }, {
            projectId: this.currentProjectId ?? undefined
        });

        // Reload projects to refresh cache
        await this.loadProjects();

        return response;
    }

    async updateProject(id: string, name: string, framework: Framework, dropHealthyHealthchecks: boolean, healthcheckPaths: string[]): Promise<Project> {
        const response = await api.put('/projects', { name, framework, dropHealthyHealthchecks, healthcheckPaths }, {
            projectId: id
        });
        await this.loadProjects();
        return response;
    }

    async deleteProject(id: string, name: string): Promise<void> {
        await api.delete('/projects', { projectId: id }, { name });
        await this.loadProjects();
    }

    async generateSourceMapToken(): Promise<string> {
        const resp = await api.post('/projects/source-map-token', {}, {
            projectId: this.currentProjectId ?? undefined
        });
        await this.loadProjects();
        return resp.sourceMapToken;
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
