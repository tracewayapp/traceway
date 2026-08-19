import { api } from '$lib/api';
import { projectsState } from '$lib/state/projects.svelte';

export type CheckType = 'http' | 'tcp' | 'browser';
export type CheckStatus = 'up' | 'down' | 'unknown';

export interface SyntheticCheck {
	id: number;
	projectId: string;
	name: string;
	checkType: CheckType;
	enabled: boolean;
	intervalSeconds: number;
	timeoutSeconds: number;
	config: CheckConfig;
	failureThreshold: number;
	currentStatus: CheckStatus;
	consecutiveFailures: number;
	lastRunAt: string | null;
	lastStateChangeAt: string | null;
	nextRunAt: string;
	createdAt: string;
	updatedAt: string;
}

export interface CheckConfig {
	// http
	url?: string;
	method?: string;
	headers?: Record<string, string>;
	auth?: {
		type?: 'none' | 'basic' | 'bearer' | 'header';
		username?: string;
		password?: string;
		token?: string;
		headerName?: string;
		headerValue?: string;
	};
	body?: string;
	bodyContentType?: string;
	followRedirects?: boolean;
	skipTlsVerify?: boolean;
	assertions?: {
		statusCodes?: number[];
		bodyContains?: string;
		bodyRegex?: string;
		maxResponseTimeMs?: number;
		tlsMinDaysRemaining?: number;
	};
	// tcp
	host?: string;
	port?: number;
	useTls?: boolean;
	sendPayload?: string;
	expectContains?: string;
	tlsMinDaysRemaining?: number;
	// browser
	script?: string;
	env?: Record<string, string>;
}

export interface CheckAggregates {
	total: number;
	up: number;
	down: number;
	missed: number;
	avgLatencyMs: number;
}

export interface CheckOverview extends SyntheticCheck {
	aggregates: CheckAggregates | null;
}

export interface CheckIncident {
	id: number;
	checkId: number | null;
	projectId: string | null;
	statusPageId: number | null;
	title: string;
	startedAt: string;
	resolvedAt: string | null;
	errorMessage: string;
	updatesCount?: number;
	postMortemId?: number | null;
}

export type IncidentUpdateStatus =
	| 'investigating'
	| 'identified'
	| 'monitoring'
	| 'update'
	| 'resolved';

export interface IncidentUpdate {
	id: number;
	incidentId: number;
	status: IncidentUpdateStatus;
	message: string;
	createdBy: number | null;
	createdAt: string;
}

export interface OrgIncident extends CheckIncident {
	checkName: string | null;
	statusPageName: string | null;
}

export function incidentDisplayTitle(incident: {
	title?: string | null;
	checkName?: string | null;
}): string {
	if (incident.title) return incident.title;
	return incident.checkName ? `${incident.checkName} is down` : 'Incident';
}

export interface PostMortemListItem {
	id: number;
	organizationId: number;
	incidentId: number | null;
	title: string;
	tags: string[];
	createdBy: number | null;
	createdAt: string;
	updatedAt: string;
}

export interface PostMortem extends PostMortemListItem {
	contentMd: string;
}

export interface CheckResultEntry {
	runId: number;
	checkType: CheckType;
	recordedAt: string;
	executedAt: string;
	status: 'up' | 'down' | 'missed';
	latencyMs: number;
	statusCode: number;
	errorMessage: string;
	executedBy: string;
	screenshotKey: string;
	outputKey: string;
	tlsDaysRemaining: number;
}

export interface CheckSeriesPoint {
	bucket: string;
	total: number;
	up: number;
	missed: number;
	avgLatencyMs: number;
	maxLatencyMs: number;
}

class MonitorsState {
	downChecksCount = $state(0);

	async refreshDownCount() {
		try {
			const res = await api.get('/synthetics/open-count', {
				projectId: projectsState.currentProjectId ?? undefined
			});
			this.downChecksCount = res.count ?? 0;
		} catch {
			// badge stays at its last value
		}
	}
}

export const monitorsState = new MonitorsState();
