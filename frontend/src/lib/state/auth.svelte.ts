import { clearNavDepth } from '$lib/utils/back-navigation';

export interface UserOrganizationResponse {
    id: number;
    name: string;
    role: string;
    timezone: string;
}

class AuthState {
    token = $state<string | null>(localStorage.getItem('AUTH_TOKEN'));
    organizations = $state<UserOrganizationResponse[]>(
        JSON.parse(localStorage.getItem('USER_ORGANIZATIONS') || '[]')
    );

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
            $effect(() => {
                if (this.organizations.length > 0) {
                    localStorage.setItem('USER_ORGANIZATIONS', JSON.stringify(this.organizations));
                } else {
                    localStorage.removeItem('USER_ORGANIZATIONS');
                }
            });
        });
    }

    setToken(token: string) {
        this.token = token;
    }

    setOrganizations(organizations: UserOrganizationResponse[]) {
        this.organizations = organizations;
    }

    getRoleForOrganization(organizationId: number): string | null {
        const organization = this.organizations.find(o => o.id === organizationId);
        return organization?.role || null;
    }

    canManageOrganization(organizationId: number): boolean {
        const role = this.getRoleForOrganization(organizationId);
        return role === 'owner' || role === 'admin';
    }

    getTimezoneForOrganization(organizationId: number): string | null {
        const organization = this.organizations.find(o => o.id === organizationId);
        return organization?.timezone || null;
    }

    updateOrganizationTimezone(organizationId: number, timezone: string) {
        const organization = this.organizations.find(o => o.id === organizationId);
        if (organization) {
            organization.timezone = timezone;
            this.organizations = [...this.organizations];
        }
    }

    logout() {
        this.token = null;
        this.organizations = [];
        clearNavDepth();
    }
}

export const authState = new AuthState();
