import { normalizeBaseUrl, selfHostedBaseUrl } from './connection-string';

export interface SetupPlanDeployment {
	platform: string;
	instructions: string;
}

export interface SetupPlanProject {
	name: string;
	framework: string;
	envFile?: string;
	envVar?: string;
	/**
	 * `connectionString` puts `<token>@<server>/api/report` in one variable: the 1.x form of old plans, and what a
	 * `react-native` project gets when the field is left out (see `envFormat`).
	 */
	envFormat?: 'token' | 'connectionString';
	urlEnvVar?: string;
	uploadTokenEnvFile?: string;
	uploadTokenEnvVar?: string;
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
	backendUrl?: string;
	projects?: SetupDraftProject[];
}

export interface SetupTokenResponse {
	token: string;
	prefix: string;
	expiresAt: string;
	organizationId: number;
	backendUrl: string;
}

/**
 * The form the token variable takes. `@tracewayapp/react-native` starts only from a connection string, so a React
 * Native project gets one unless the plan says otherwise, as in `traceway setup apply` and on the server.
 */
export function envFormat(planned: SetupPlanProject): 'token' | 'connectionString' {
	if (planned.envFormat) return planned.envFormat;
	return planned.framework === 'react-native' ? 'connectionString' : 'token';
}

/** The value the token variable gets: the token, or `<token>@<server>/api/report` for a connection string. */
export function credentialValue(planned: SetupPlanProject, project: SetupDraftProject): string {
	if (envFormat(planned) === 'connectionString') {
		const base = normalizeBaseUrl(project.backendUrl) ?? project.backendUrl.replace(/\/+$/, '');
		return `${project.token}@${base}/api/report`;
	}
	return project.token;
}

/** The server URL variable `traceway setup` writes next to the token off Traceway Cloud (null on Cloud). */
export function urlEnvVar(
	planned: SetupPlanProject,
	backendUrl: string | null | undefined
): string | null {
	if (envFormat(planned) === 'connectionString' || !planned.envVar) return null;
	if (!backendUrl || selfHostedBaseUrl(backendUrl) === null) return null;
	if (planned.urlEnvVar) return planned.urlEnvVar;
	const name = planned.envVar;
	if (name.endsWith('TOKEN')) return name.slice(0, -'TOKEN'.length) + 'URL';
	if (name.endsWith('token')) return name.slice(0, -'token'.length) + 'url';
	return name.includes('.') ? `${name}.url` : `${name}_URL`;
}

const BROWSER_FRAMEWORKS = new Set(['react', 'svelte', 'vuejs', 'jquery', 'nextjs']);

/** The variable the release build reads the upload token from: the browser source map uploader's, or the others'. */
export function uploadTokenEnvVar(planned: SetupPlanProject): string {
	if (planned.uploadTokenEnvVar) return planned.uploadTokenEnvVar;
	return BROWSER_FRAMEWORKS.has(planned.framework)
		? 'TRACEWAY_SOURCEMAP_TOKEN'
		: 'TRACEWAY_UPLOAD_TOKEN';
}

/** Where `traceway setup` writes the upload token, or null when the plan leaves it on the Connection page. */
export function uploadTokenEnvFile(planned: SetupPlanProject): string | null {
	if (planned.uploadTokenEnvFile) return planned.uploadTokenEnvFile;
	if (planned.uploadTokenEnvVar && planned.envFile) return planned.envFile;
	return null;
}

/** Fills a deployment command: `<token>` is the credential, `<url>` the server's base URL. */
export function substituteToken(instructions: string, value: string, backendUrl?: string): string {
	let result = instructions.replaceAll('<token>', value);
	if (backendUrl !== undefined) {
		result = result.replaceAll('<url>', normalizeBaseUrl(backendUrl) ?? backendUrl);
	}
	return result;
}
