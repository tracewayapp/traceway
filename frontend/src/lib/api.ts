import { authState } from './state/auth.svelte';
import { toast } from 'svelte-sonner';

const BASE_URL = '/api';

interface RequestOptions {
    projectId?: string;
    skipProjectId?: boolean;
}

async function request(method: string, endpoint: string, data?: unknown, options?: RequestOptions) {
    const currentToken = authState.token;
    const headers: Record<string, string> = {
        'Content-Type': 'application/json'
    };

    if (currentToken) {
        headers['Authorization'] = `Bearer ${currentToken}`;
    }

    const config: RequestInit = {
        method,
        headers,
    };

    if (data) {
        config.body = JSON.stringify(data);
    }

    // Add projectId as query parameter if provided
    let url = `${BASE_URL}${endpoint}`;
    if (options?.projectId && !options?.skipProjectId) {
        const separator = endpoint.includes('?') ? '&' : '?';
        url = `${url}${separator}projectId=${options.projectId}`;
    }

    const response = await fetch(url, config);

    if (response.status === 401) {
        authState.logout();
        window.location.href = '/login';
        throw new Error('Unauthorized');
    }

    if (response.status === 403) {
        const currentPath = window.location.pathname;
        if (currentPath === '/' || currentPath === '') {
            authState.logout();
            window.location.href = '/login';
        } else {
            toast.warning("You don't have permission to access that feature", { position: 'top-center' });
            window.location.href = '/';
        }
        throw new Error('Forbidden');
    }

    if (!response.ok) {
        const error = new Error(`API Error: ${response.statusText}`) as Error & { status: number };
        error.status = response.status;
        throw error;
    }

    return response.json();
}

export const api = {
    get: (endpoint: string, options?: RequestOptions) => request('GET', endpoint, undefined, options),
    post: (endpoint: string, data: unknown, options?: RequestOptions) => request('POST', endpoint, data, options),
    put: (endpoint: string, data: unknown, options?: RequestOptions) => request('PUT', endpoint, data, options),
    delete: (endpoint: string, options?: RequestOptions) => request('DELETE', endpoint, undefined, options)
};
