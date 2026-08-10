import { api } from '$lib/api';
import { authState } from './auth.svelte';
import { projectsState } from './projects.svelte';

export interface TeamMember {
    teamId: number;
    userId: number;
    position: number;
    name: string;
    email: string;
}

export interface TeamProject {
    projectId: string;
    name: string;
}

export interface Team {
    id: number;
    organizationId: number;
    name: string;
    description: string;
    createdAt: string;
    updatedAt: string;
    memberCount: number;
    projectCount: number;
    scheduleCount: number;
    members: TeamMember[];
    projects: TeamProject[];
}

export interface ScheduleRestriction {
    type: 'daily' | 'weekly';
    startTime: string;
    endTime: string;
    startDay?: number;
    endDay?: number;
}

export interface ScheduleLayer {
    id?: string;
    name: string;
    rotationType: 'daily' | 'weekly' | 'custom';
    handoffTime: string;
    handoffDay?: number;
    intervalDays?: number;
    rotationStart: string;
    userIds: number[];
    restrictions: ScheduleRestriction[];
}

export interface ScheduleDefinition {
    schemaVersion: number;
    layers: ScheduleLayer[];
}

export interface Schedule {
    id: number;
    organizationId: number;
    teamId: number;
    name: string;
    description: string;
    timezone: string;
    definition: ScheduleDefinition;
    createdBy: number;
    createdAt: string;
    updatedAt: string;
}

export interface ScheduleOverride {
    id: number;
    scheduleId: number;
    userId: number;
    startAt: string;
    endAt: string;
    createdBy: number;
    createdAt: string;
}

export interface TimelineShift {
    userId: number;
    layerId: string;
    start: string;
    end: string;
    isOverride: boolean;
}

export interface TimelineLayer {
    id: string;
    name: string;
    shifts: TimelineShift[];
}

export interface TimelineUser {
    name: string;
    email: string;
}

export interface TimelineResponse {
    schedule: { id: number; name: string; timezone: string; teamId: number };
    from: string;
    to: string;
    layers: TimelineLayer[];
    final: TimelineShift[];
    users: Record<string, TimelineUser | null>;
}

export interface OncallUser {
    userId: number;
    name: string;
    email: string;
}

export interface OverviewSchedule {
    id: number;
    name: string;
    oncall: OncallUser[];
    until: string | null;
    nextUp: OncallUser | null;
    nextAt: string | null;
}

export interface OverviewTeam {
    team: {
        id: number;
        name: string;
        description: string;
        memberCount: number;
        projectCount: number;
        scheduleCount: number;
    };
    schedules: OverviewSchedule[];
}

export interface ProjectOncall {
    team: { id: number; name: string } | null;
    schedules: { id: number; name: string }[];
    oncall: OncallUser[];
}

export interface OrgMember {
    id: number;
    email: string;
    name: string;
    role: string;
}

export type PolicyTargetType = 'schedule' | 'user' | 'team' | 'channel';

export interface PolicyTarget {
    type: PolicyTargetType;
    id: number;
}

export interface PolicyStep {
    targets: PolicyTarget[];
    delayMinutes: number;
}

export interface PolicyDefinition {
    schemaVersion: number;
    steps: PolicyStep[];
    repeatCount: number;
    urgency?: 'auto' | 'high' | 'low' | '';
}

export interface EscalationPolicy {
    id: number;
    organizationId: number;
    name: string;
    definition: PolicyDefinition;
    createdBy: number;
    createdAt: string;
    updatedAt: string;
}

export interface OncallPage {
    id: number;
    organizationId: number;
    projectId: string;
    policyId: number;
    policySnapshot: PolicyDefinition;
    ruleId: number;
    ruleName: string;
    ruleType: string;
    subject: string;
    body: string;
    url: string;
    severity: string;
    urgency: '' | 'high' | 'low';
    status: 'open' | 'acknowledged' | 'resolved';
    eventCount: number;
    lastEventAt: string;
    escalationLevel: number;
    repeatIteration: number;
    nextEscalationAt: string | null;
    lastEscalatedAt: string | null;
    acknowledgedBy: number | null;
    acknowledgedAt: string | null;
    acknowledgedVia: '' | 'dashboard' | 'link';
    resolvedBy: number | null;
    resolvedAt: string | null;
    createdAt: string;
    updatedAt: string;
}

export interface PageNotification {
    id: number;
    pageId: number;
    level: number;
    iteration: number;
    userId: number | null;
    targetDesc: string;
    methodType: string;
    status: 'pending' | 'sent' | 'failed' | 'cancelled';
    errorMsg: string;
    createdAt: string;
    sentAt: string | null;
    scheduledFor: string | null;
}

export interface PageDetailResponse {
    page: OncallPage;
    notifications: PageNotification[];
    users: Record<string, string>;
}

export interface ContactMethod {
    id: number;
    userId: number;
    methodType: 'email' | 'slack' | 'pushover' | 'telegram' | 'sms';
    config: Record<string, string>;
    enabled: boolean;
    verified: boolean;
    createdAt: string;
}

export interface UserNotificationRuleStep {
    id?: number;
    contactMethodId: number;
    delayMinutes: number;
}

export function getCurrentUserFromToken(): { userId: number; email: string } | null {
    const token = authState.token;
    if (!token) return null;
    try {
        const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
        if (typeof payload.userId !== 'number') return null;
        return { userId: payload.userId, email: payload.email ?? '' };
    } catch {
        return null;
    }
}

class OncallState {
    teams = $state<Team[]>([]);
    teamsLoading = $state(false);
    schedules = $state<Schedule[]>([]);
    schedulesLoading = $state(false);
    overview = $state<OverviewTeam[]>([]);
    overviewLoading = $state(false);
    policies = $state<EscalationPolicy[]>([]);
    policiesLoading = $state(false);
    openPagesCount = $state(0);

    canManage(organizationId: number): boolean {
        return authState.canManageOrganization(organizationId);
    }

    async refreshOpenCount() {
        try {
            const res = await api.get('/pages/open-count', {
                projectId: projectsState.currentProjectId ?? undefined
            });
            this.openPagesCount = res.count ?? 0;
        } catch {
            // badge stays at its last value
        }
    }

    async loadTeams(organizationId: number) {
        this.teamsLoading = true;
        try {
            const res = await api.get(`/organizations/${organizationId}/teams`);
            this.teams = res.teams || [];
        } catch {
            this.teams = [];
        } finally {
            this.teamsLoading = false;
        }
    }

    async loadSchedules(organizationId: number) {
        this.schedulesLoading = true;
        try {
            const res = await api.get(`/organizations/${organizationId}/schedules`);
            this.schedules = res.schedules || [];
        } catch {
            this.schedules = [];
        } finally {
            this.schedulesLoading = false;
        }
    }

    async loadOverview(organizationId: number) {
        this.overviewLoading = true;
        try {
            const res = await api.get(`/organizations/${organizationId}/oncall/now`);
            this.overview = res.teams || [];
        } catch {
            this.overview = [];
        } finally {
            this.overviewLoading = false;
        }
    }

    async createTeam(
        organizationId: number,
        body: { name: string; description: string; memberUserIds: number[]; projectIds: string[] }
    ) {
        const team = await api.post(`/organizations/${organizationId}/teams`, body);
        await this.loadTeams(organizationId);
        return team;
    }

    async updateTeam(
        organizationId: number,
        teamId: number,
        body: { name: string; description: string }
    ) {
        await api.put(`/organizations/${organizationId}/teams/${teamId}`, body);
    }

    async updateTeamMembers(organizationId: number, teamId: number, userIds: number[]) {
        await api.put(`/organizations/${organizationId}/teams/${teamId}/members`, { userIds });
    }

    async updateTeamProjects(organizationId: number, teamId: number, projectIds: string[]) {
        await api.put(`/organizations/${organizationId}/teams/${teamId}/projects`, { projectIds });
    }

    async deleteTeam(organizationId: number, teamId: number) {
        await api.delete(`/organizations/${organizationId}/teams/${teamId}`);
        await this.loadTeams(organizationId);
    }

    async createSchedule(
        organizationId: number,
        body: {
            teamId: number;
            name: string;
            description: string;
            timezone: string;
            definition: ScheduleDefinition;
        }
    ) {
        const schedule = await api.post(`/organizations/${organizationId}/schedules`, body);
        await this.loadSchedules(organizationId);
        return schedule;
    }

    async updateSchedule(
        organizationId: number,
        scheduleId: number,
        body: {
            teamId: number;
            name: string;
            description: string;
            timezone: string;
            definition: ScheduleDefinition;
        }
    ) {
        await api.put(`/organizations/${organizationId}/schedules/${scheduleId}`, body);
        await this.loadSchedules(organizationId);
    }

    async deleteSchedule(organizationId: number, scheduleId: number) {
        await api.delete(`/organizations/${organizationId}/schedules/${scheduleId}`);
        await this.loadSchedules(organizationId);
    }

    async getSchedule(
        organizationId: number,
        scheduleId: number
    ): Promise<{ schedule: Schedule; overrides: ScheduleOverride[] }> {
        return await api.get(`/organizations/${organizationId}/schedules/${scheduleId}`);
    }

    async getTimeline(
        organizationId: number,
        scheduleId: number,
        from: string,
        to: string
    ): Promise<TimelineResponse> {
        return await api.get(
            `/organizations/${organizationId}/schedules/${scheduleId}/timeline?from=${encodeURIComponent(from)}&to=${encodeURIComponent(to)}`
        );
    }

    async createOverride(
        organizationId: number,
        scheduleId: number,
        body: { userId: number; startAt: string; endAt: string }
    ) {
        return await api.post(`/organizations/${organizationId}/schedules/${scheduleId}/overrides`, body);
    }

    async deleteOverride(organizationId: number, scheduleId: number, overrideId: number) {
        await api.delete(
            `/organizations/${organizationId}/schedules/${scheduleId}/overrides/${overrideId}`
        );
    }

    async getMembers(organizationId: number): Promise<OrgMember[]> {
        return await api.get(`/organizations/${organizationId}/members`);
    }

    async loadPolicies(organizationId: number) {
        this.policiesLoading = true;
        try {
            const res = await api.get(`/organizations/${organizationId}/escalation-policies`);
            this.policies = res.policies || [];
        } catch {
            this.policies = [];
        } finally {
            this.policiesLoading = false;
        }
    }

    async createPolicy(organizationId: number, body: { name: string; definition: PolicyDefinition }) {
        const policy = await api.post(`/organizations/${organizationId}/escalation-policies`, body);
        await this.loadPolicies(organizationId);
        return policy;
    }

    async updatePolicy(
        organizationId: number,
        policyId: number,
        body: { name: string; definition: PolicyDefinition }
    ) {
        await api.put(`/organizations/${organizationId}/escalation-policies/${policyId}`, body);
        await this.loadPolicies(organizationId);
    }

    async deletePolicy(organizationId: number, policyId: number) {
        await api.delete(`/organizations/${organizationId}/escalation-policies/${policyId}`);
        await this.loadPolicies(organizationId);
    }
}

export const oncallState = new OncallState();
