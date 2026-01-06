import { api } from '$lib/api';

export type Framework = 'gin' | 'fiber' | 'chi' | 'fasthttp' | 'stdlib' | 'custom';

export interface Project {
    id: string;
    name: string;
    framework: Framework;
    createdAt: string;
}

export interface ProjectWithToken extends Project {
    token: string;
}

class ProjectsState {
    projects = $state<Project[]>([]);
    currentProjectId = $state<string | null>(localStorage.getItem('CURRENT_PROJECT_ID'));
    loading = $state(false);
    error = $state<string | null>(null);

    currentProject = $derived(
        this.projects.find(p => p.id === this.currentProjectId) || this.projects[0] || null
    );

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

    async loadProjects() {
        this.loading = true;
        this.error = null;

        try {
            const response = await api.get('/projects');
            this.projects = response;

            // If no current project selected or current project not in list, select first one
            if (!this.currentProjectId || !this.projects.find(p => p.id === this.currentProjectId)) {
                if (this.projects.length > 0) {
                    this.currentProjectId = this.projects[0].id;
                }
            }

            // Cache in localStorage
            localStorage.setItem('PROJECTS_CACHE', JSON.stringify(this.projects));
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

    async getProjectWithToken(id: string): Promise<ProjectWithToken> {
        return await api.get(`/projects/${id}`);
    }

    selectProject(projectId: string) {
        this.currentProjectId = projectId;
    }

    // Initialize from localStorage cache on startup
    initFromCache() {
        const cached = localStorage.getItem('PROJECTS_CACHE');
        if (cached) {
            this.projects = JSON.parse(cached);
        }
    }
}

export const projectsState = new ProjectsState();
