class AuthState {
    token = $state<string | null>(localStorage.getItem('APP_TOKEN'));

    isAuthenticated = $derived(!!this.token);

    constructor() {
        $effect.root(() => {
            $effect(() => {
                if (this.token) {
                    localStorage.setItem('APP_TOKEN', this.token);
                } else {
                    localStorage.removeItem('APP_TOKEN');
                }
            });
        });
    }

    setToken(token: string) {
        this.token = token;
    }

    logout() {
        this.token = null;
    }
}

export const authState = new AuthState();
