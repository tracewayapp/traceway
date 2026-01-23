import { api } from '$lib/api';

export interface Organization {
    id: number;
    name: string;
    timezone: string;
    createdAt: string;
}

export interface OrganizationMember {
    id: number;
    email: string;
    name: string;
    role: string;
    createdAt: string;
}

export interface Invitation {
    id: number;
    organizationId: number;
    email: string;
    role: string;
    invitedBy: number;
    inviterName: string;
    status: string;
    expiresAt: string;
    acceptedAt?: string;
    createdAt: string;
}

export interface OrganizationSettings {
    organization: Organization;
    members: OrganizationMember[];
    invitations: Invitation[];
    userRole: string;
}

class OrganizationState {
    currentOrganization = $state<Organization | null>(null);
    members = $state<OrganizationMember[]>([]);
    invitations = $state<Invitation[]>([]);
    loading = $state(false);
    error = $state<string | null>(null);
    userRole = $state<string | null>(null);

    isOwner = $derived(this.userRole === 'owner');
    isAdmin = $derived(this.userRole === 'admin');
    canManage = $derived(this.userRole === 'owner' || this.userRole === 'admin');
    memberCount = $derived(this.members.length + this.invitations.filter(i => i.status === 'pending').length);
    canInvite = $derived(this.memberCount < 10);

    async loadSettings(organizationId: number) {
        this.loading = true;
        this.error = null;

        try {
            const response: OrganizationSettings = await api.get(`/organizations/${organizationId}/settings`);
            this.currentOrganization = response.organization;
            this.members = response.members;
            this.invitations = response.invitations;
            this.userRole = response.userRole;
        } catch (e: unknown) {
            const errorMessage = e instanceof Error ? e.message : 'Failed to load settings';
            this.error = errorMessage;
        } finally {
            this.loading = false;
        }
    }

    async inviteUser(organizationId: number, email: string, role: string) {
        const response = await api.post(`/organizations/${organizationId}/invitations`, { email, role });
        await this.loadSettings(organizationId);
        return response;
    }

    async revokeInvitation(organizationId: number, invitationId: number) {
        await api.delete(`/organizations/${organizationId}/invitations/${invitationId}`);
        await this.loadSettings(organizationId);
    }

    async updateMemberRole(organizationId: number, userId: number, role: string) {
        await api.put(`/organizations/${organizationId}/members/${userId}`, { role });
        await this.loadSettings(organizationId);
    }

    async removeMember(organizationId: number, userId: number) {
        await api.delete(`/organizations/${organizationId}/members/${userId}`);
        await this.loadSettings(organizationId);
    }

    async updateTimezone(organizationId: number, timezone: string) {
        await api.put(`/organizations/${organizationId}/settings`, { timezone });
        if (this.currentOrganization) {
            this.currentOrganization = { ...this.currentOrganization, timezone };
        }
    }

    clear() {
        this.currentOrganization = null;
        this.members = [];
        this.invitations = [];
        this.userRole = null;
        this.error = null;
    }
}

export const organizationState = new OrganizationState();
