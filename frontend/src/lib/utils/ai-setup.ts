export const SKILL_INSTALL_COMMAND = 'npx skills add tracewayapp/traceway';

export function getSetupPrompt(backendUrl: string, token: string): string {
	return `/traceway-setup with token ${token} and url ${backendUrl}`;
}
