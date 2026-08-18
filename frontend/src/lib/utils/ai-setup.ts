export const SKILL_INSTALL_COMMAND = 'npx skills add tracewayapp/traceway';

export type PromptPart = {
	text: string;
	bold: boolean;
};

// Prompt for connecting an EXISTING project: carries the project's ingest
// token (and optionally its source map upload token).
export function getSetupPromptParts(
	backendUrl: string,
	token: string,
	sourceMapToken: string | null = null
): PromptPart[] {
	const parts: PromptPart[] = [
		{ text: '/traceway-setup with token ', bold: false },
		{ text: token, bold: true },
		{ text: ' and url ', bold: false },
		{ text: backendUrl, bold: true }
	];
	if (sourceMapToken) {
		parts.push(
			{ text: ' and source map upload token ', bold: false },
			{ text: sourceMapToken, bold: true }
		);
	}
	return parts;
}

export function getSetupPrompt(
	backendUrl: string,
	token: string,
	sourceMapToken: string | null = null
): string {
	return getSetupPromptParts(backendUrl, token, sourceMapToken)
		.map((p) => p.text)
		.join('');
}

// Prompt for creating NEW projects: carries an org-scoped tws_ setup token.
// The agent proposes a plan the user approves on this website; the literal
// words "setup token" (plus the tws_ prefix) are what the skill keys on.
export function getSetupTokenPromptParts(backendUrl: string, setupToken: string): PromptPart[] {
	return [
		{ text: '/traceway-setup with setup token ', bold: false },
		{ text: setupToken, bold: true },
		{ text: ' and url ', bold: false },
		{ text: backendUrl, bold: true }
	];
}

export function getSetupTokenPrompt(backendUrl: string, setupToken: string): string {
	return getSetupTokenPromptParts(backendUrl, setupToken)
		.map((p) => p.text)
		.join('');
}
