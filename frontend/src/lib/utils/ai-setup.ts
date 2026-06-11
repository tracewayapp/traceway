export const SKILL_INSTALL_COMMAND = 'npx skills add tracewayapp/traceway';

export type PromptPart = {
	text: string;
	bold: boolean;
};

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
