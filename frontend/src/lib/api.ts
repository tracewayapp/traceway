import { authState } from './state/auth.svelte';

const BASE_URL = '/api';

async function request(method: string, endpoint: string, data?: any) {
    const currentToken = authState.token;
    const headers: Record<string, string> = {
        'Content-Type': 'application/json'
    };

    if (currentToken) {
        headers['Authorization'] = `${currentToken}`;
    }

    const config: RequestInit = {
        method,
        headers,
    };

    if (data) {
        config.body = JSON.stringify(data);
    }

    const response = await fetch(`${BASE_URL}${endpoint}`, config);

    if (response.status === 401) {
        authState.logout();
        window.location.href = '/login';
        throw new Error('Unauthorized');
    }

    if (!response.ok) {
        throw new Error(`API Error: ${response.statusText}`);
    }

    return response.json();
}

export const api = {
    get: (endpoint: string) => request('GET', endpoint),
    post: (endpoint: string, data: any) => request('POST', endpoint, data),
    put: (endpoint: string, data: any) => request('PUT', endpoint, data),
    delete: (endpoint: string) => request('DELETE', endpoint)
};
