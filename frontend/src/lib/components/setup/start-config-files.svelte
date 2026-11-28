<script lang="ts">
	import bash from 'svelte-highlight/languages/bash';
	import ini from 'svelte-highlight/languages/ini';
	import kotlin from 'svelte-highlight/languages/kotlin';
	import properties from 'svelte-highlight/languages/properties';
	import xml from 'svelte-highlight/languages/xml';
	import type { LanguageFn } from 'highlight.js';
	import type { StartConfigFile, StartLanguage } from '$lib/utils/sdk-start';
	import CopyableCodeBlock from './copyable-code-block.svelte';

	let { files }: { files: StartConfigFile[] } = $props();

	const languages: Record<StartLanguage, { name: string; register: LanguageFn }> = {
		bash,
		ini,
		kotlin,
		properties,
		xml
	};
</script>

{#each files as file (file.title)}
	<div>
		<p class="text-sm font-medium">
			{#if /\s/.test(file.title)}{file.title}{:else}<code class="font-mono">{file.title}</code>{/if}
		</p>
		<p class="mb-2 text-xs text-muted-foreground">{file.description}</p>
		<CopyableCodeBlock code={file.code} language={languages[file.language]} secret={file.secret} />
	</div>
{/each}
