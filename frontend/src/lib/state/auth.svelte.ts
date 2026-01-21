import { clearNavDepth } from '$lib/utils/back-navigation';
import { userState } from './user.svelte';

class AuthState {
    token = $state<string | null>(localStorage.getItem('AUTH_TOKEN'));

    isAuthenticated = $derived(!!this.token);

    constructor() {
        $effect.root(() => {
            $effect(() => {
                if (this.token) {
                    localStorage.setItem('AUTH_TOKEN', this.token);
                } else {
                    localStorage.removeItem('AUTH_TOKEN');
                }
            });
        });
    }

    setToken(token: string) {
        this.token = token;
    }

    logout() {
        this.token = null;
        userState.clear();
        clearNavDepth();
    }
}

export const authState = new AuthState();
