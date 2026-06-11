<script lang="ts">
	import yaml from 'svelte-highlight/languages/yaml';
	import CopyableInline from './copyable-inline.svelte';
	import CopyableCodeBlock from './copyable-code-block.svelte';

	let {
		endpoint,
		authHeader,
		collectorConfig
	}: { endpoint: string; authHeader: string; collectorConfig: string } = $props();
</script>

<div class="space-y-6">
	<div>
		<p class="mb-1 text-sm font-medium">OTLP Endpoint</p>
		<p class="mb-2 text-xs text-muted-foreground">
			Your SDK or Collector will append <code
				class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/traces</code
			>
			and
			<code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/metrics</code> automatically.
		</p>
		<CopyableInline value={endpoint} />
	</div>
	<div>
		<p class="mb-2 text-sm font-medium">Authorization Header</p>
		<CopyableInline value={authHeader} />
	</div>
	<div>
		<p class="mb-2 text-sm font-medium">Example: OTel Collector (optional)</p>
		<CopyableCodeBlock code={collectorConfig} language={yaml} />
	</div>
</div>
