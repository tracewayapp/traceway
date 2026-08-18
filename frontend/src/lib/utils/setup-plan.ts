// Types for the AI setup flow: a coding agent submits a plan draft with its
// setup token; the user approves it here, which creates the projects.

export interface SetupPlanDeployment {
	platform: string;
	instructions: string;
}

export interface SetupPlanProject {
	name: string;
	framework: string;
	envFile?: string;
	envVar?: string;
	envFormat?: 'token' | 'connectionString';
	deployment?: SetupPlanDeployment;
}

export interface SetupDraftProject {
	id: string;
	name: string;
	framework: string;
	token: string;
	sourceMapToken?: string | null;
	backendUrl: string;
	status: 'created' | 'existing';
}

export interface SetupDraft {
	id: string;
	status: 'pending' | 'approved' | 'rejected';
	payload: { projects: SetupPlanProject[] };
	createdAt: string;
	requestedByEmail: string;
	rejectReason?: string;
	projects?: SetupDraftProject[];
}

export interface SetupTokenResponse {
	token: string;
	prefix: string;
	expiresAt: string;
	organizationId: number;
	backendUrl: string;
}

// The credential value a deployment instruction needs: the raw ingest token,
// or the SDK connection string when the plan says so.
export function credentialValue(planned: SetupPlanProject, project: SetupDraftProject): string {
	if (planned.envFormat === 'connectionString') {
		const base = project.backendUrl.replace(/\/+$/, '');
		return `${project.token}@${base}/api/report`;
	}
	return project.token;
}

export function substituteToken(instructions: string, value: string): string {
	return instructions.replaceAll('<token>', value);
}
