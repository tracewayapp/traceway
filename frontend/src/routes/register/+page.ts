import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = () => {
    // Check if user is already authenticated
    const token = localStorage.getItem('AUTH_TOKEN');
    if (token) {
        throw redirect(302, '/');
    }
    return {};
};
