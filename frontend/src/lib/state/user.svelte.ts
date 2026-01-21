export interface User {
    id: number;
    email: string;
    name: string;
    createdAt: string;
}

const USER_STORAGE_KEY = 'AUTH_USER';

class UserState {
    user = $state<User | null>(null);

    constructor() {
        // Load from localStorage on init
        const stored = localStorage.getItem(USER_STORAGE_KEY);
        if (stored) {
            try {
                this.user = JSON.parse(stored);
            } catch {
                this.user = null;
            }
        }

        $effect.root(() => {
            $effect(() => {
                if (this.user) {
                    localStorage.setItem(USER_STORAGE_KEY, JSON.stringify(this.user));
                } else {
                    localStorage.removeItem(USER_STORAGE_KEY);
                }
            });
        });
    }

    setUser(user: User) {
        this.user = user;
    }

    clear() {
        this.user = null;
    }

    async loadUser() {
        try {
            const response = await fetch('/api/me', {
                headers: {
                    'Authorization': `Bearer ${localStorage.getItem('AUTH_TOKEN')}`
                }
            });
            if (response.ok) {
                const user = await response.json();
                this.setUser(user);
            }
        } catch {
            // Ignore errors
        }
    }
}

export const userState = new UserState();
