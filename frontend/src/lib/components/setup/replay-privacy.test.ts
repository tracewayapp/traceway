import { afterEach, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/svelte';
import { tick } from 'svelte';
import { record } from '@rrweb/record';
import bash from 'svelte-highlight/languages/bash';
import CopyableInline from './copyable-inline.svelte';
import CopyableCodeBlock from './copyable-code-block.svelte';
import CopyablePrompt from './copyable-prompt.svelte';
import KeyValueRows from '$lib/components/synthetics/key-value-rows.svelte';

afterEach(cleanup);

// The SDK passes rrweb nothing but maskAllInputs, which remote config may switch off, so record the way the
// dashboard's own replay would at worst.
function startRecording(): () => string {
	const events: unknown[] = [];
	const stop = record({ emit: (event) => events.push(event), maskAllInputs: false });
	return () => {
		stop?.();
		return JSON.stringify(events);
	};
}

function recordPage(): string {
	return startRecording()();
}

it('keeps a secret inline value out of the replay', () => {
	render(CopyableInline, { value: 'tww_inline_secret', secret: true });
	render(CopyableInline, { value: 'https://traceway.example.com/api/otel' });
	const replay = recordPage();
	expect(replay).not.toContain('tww_inline_secret');
	expect(replay).toContain('https://traceway.example.com/api/otel');
});

it('keeps a token that appears after recording started out of the replay', async () => {
	const finish = startRecording();
	render(CopyableInline, { value: 'twp_created_later', secret: true });
	render(CopyableInline, { value: 'visible_created_later' });
	await tick();
	await new Promise((resolve) => setTimeout(resolve, 0));
	const replay = finish();
	expect(replay).not.toContain('twp_created_later');
	expect(replay).toContain('visible_created_later');
});

it('keeps a secret code snippet out of the replay', () => {
	render(CopyableCodeBlock, {
		code: 'OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer project_token_secret',
		language: bash,
		secret: true
	});
	render(CopyableCodeBlock, { code: 'npm install visible-package', language: bash });
	const replay = recordPage();
	expect(replay).not.toContain('project_token_secret');
	expect(replay).toContain('visible-package');
});

it('keeps setup prompts out of the replay', () => {
	render(CopyablePrompt, {
		parts: [
			{ text: '/traceway-setup with token ', bold: false },
			{ text: 'prompt_token_secret', bold: true }
		]
	});
	expect(recordPage()).not.toContain('prompt_token_secret');
});

it('keeps secret header values out of the replay even with inputs unmasked', () => {
	render(KeyValueRows, {
		rows: [{ key: 'X-Visible-Header', value: 'header_value_secret' }],
		secretValues: true
	});
	const replay = recordPage();
	expect(replay).not.toContain('header_value_secret');
	expect(replay).toContain('X-Visible-Header');
});
