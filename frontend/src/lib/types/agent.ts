export type AttemptStatus =
	| 'pending_approval'
	| 'queued'
	| 'claimed'
	| 'preparing'
	| 'running'
	| 'verifying'
	| 'publishing'
	| 'needs_input'
	| 'awaiting_review'
	| 'analyzed'
	| 'merged'
	| 'closed'
	| 'failed'
	| 'cancelled'
	| 'timed_out';

export interface Attempt {
	id: string;
	organizationId: number;
	projectId: string;
	repositoryId: number | null;
	profileId: number | null;
	number: number;
	kind: string;
	subjectKind: string;
	subjectRef: string;
	executor: string;
	status: AttemptStatus;
	resume: boolean;
	baseBranch: string;
	fixBranch: string;
	agent: string;
	model: string;
	costUsd: number;
	inputTokens: number;
	outputTokens: number;
	turns: number;
	claimedBy: string;
	leaseExpiresAt: string | null;
	requestedBy: number | null;
	approvedBy: number | null;
	error: string;
	reportKey: string;
	createdAt: string;
	startedAt: string | null;
	finishedAt: string | null;
	updatedAt: string;
}

export interface AttemptLink {
	id: number;
	attemptId: string;
	integrationId: number | null;
	provider: string;
	kind: string;
	externalRef: string;
	url: string;
	createdAt: string;
}

export interface AttemptWithLinks extends Attempt {
	links: AttemptLink[];
}

export interface AttemptEvent {
	id: number;
	attemptId: string;
	seq: number;
	kind: string;
	payload: Record<string, unknown>;
	createdAt: string;
}

export interface AgentMessage {
	id: number;
	attemptId: string;
	direction: 'in' | 'out';
	provider: string;
	linkId: number | null;
	identityId: number | null;
	kind: string;
	body: string;
	externalRef: string;
	deliveredAt: string | null;
	createdAt: string;
}

export interface PreflightCheck {
	key: 'repository' | 'profile' | 'executor' | 'channel';
	ok: boolean;
	label: string;
	hint?: string;
	href?: string;
}

export interface Preflight {
	checks: PreflightCheck[];
	nextNumber: number;
	activeAttempt?: Attempt | null;
	profiles: AgentProfile[];
	defaultProfileId?: number;
	canStart: boolean;
}

export interface AgentProfile {
	id: number;
	organizationId: number;
	name: string;
	agent: string;
	package: string;
	packageVersion: string;
	model: string;
	provider: string;
	baseUrl: string;
	maxTurns: number;
	timeoutMinutes: number;
	budgetUsd: number;
	allowedTools: string[];
	networkPolicy: unknown;
	isDefault: boolean;
	hasCredential?: boolean;
	createdAt: string;
	updatedAt: string;
}

export interface ProviderField {
	key: string;
	label: string;
	kind: 'text' | 'secret' | 'select' | 'url';
	help?: string;
	options?: string[];
	required: boolean;
}

export interface ProviderInfo {
	provider: string;
	kinds: string[];
	fields: ProviderField[];
	setupFlow?: { label: string; url: string } | null;
}

export interface CodeHostOption {
	id: number;
	name: string;
	provider: string;
	status?: string;
}

export interface MemberIdentity {
	provider: string;
	externalId: string;
	display: string;
}

export interface Integration {
	id: number;
	organizationId: number;
	provider: string;
	kinds: string[];
	name: string;
	config: Record<string, string>;
	hasSecrets: string[];
	enabled: boolean;
	createdAt: string;
	updatedAt: string;
}

export interface Repository {
	id: number;
	projectId: string;
	integrationId: number | null;
	owner: string;
	name: string;
	defaultBranch: string;
	image: string;
	setupCommand: string;
	testCommand: string;
	createdAt: string;
	updatedAt: string;
}
