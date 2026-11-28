import { getFrameworkLabel, type Framework } from '$lib/state/projects.svelte';

export const SKILL_INSTALL_COMMAND = 'npx skills add tracewayapp/skills --all';

export type PromptPart = {
	text: string;
	bold: boolean;
};

// Flutter, iOS and Android upload debug symbols; the browser SDKs upload source maps.
const SYMBOL_UPLOAD_FRAMEWORKS: Framework[] = ['flutter', 'android', 'ios'];

/** What the project's upload token uploads, as the prompt and the setup steps name it. */
export function uploadTokenLabel(framework: Framework | null | undefined): string {
	return framework && SYMBOL_UPLOAD_FRAMEWORKS.includes(framework)
		? 'symbol upload token'
		: 'source map upload token';
}

export function getSetupPromptParts(
	backendUrl: string,
	token: string,
	sourceMapToken: string | null = null,
	framework: Framework | null = null
): PromptPart[] {
	const parts: PromptPart[] = [{ text: '/traceway-setup ', bold: false }];
	if (framework) {
		const label = getFrameworkLabel(framework);
		parts.push(
			{ text: /^[aeiou]/i.test(label) ? 'for an ' : 'for a ', bold: false },
			{ text: label, bold: true },
			{ text: ' project ', bold: false }
		);
	}
	parts.push(
		{ text: 'with token ', bold: false },
		{ text: token, bold: true },
		{ text: ' and url ', bold: false },
		{ text: backendUrl, bold: true }
	);
	if (sourceMapToken) {
		parts.push(
			{ text: ` and ${uploadTokenLabel(framework)} `, bold: false },
			{ text: sourceMapToken, bold: true }
		);
	}
	return parts;
}

export function getSetupPrompt(
	backendUrl: string,
	token: string,
	sourceMapToken: string | null = null,
	framework: Framework | null = null
): string {
	return getSetupPromptParts(backendUrl, token, sourceMapToken, framework)
		.map((p) => p.text)
		.join('');
}

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
