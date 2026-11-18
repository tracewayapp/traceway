<script lang="ts">
	import { Code } from '@lucide/svelte';
	import { LEGACY_GO_FRAMEWORKS, type Framework } from '$lib/state/projects.svelte';
	import Highlight from 'svelte-highlight';
	import go from 'svelte-highlight/languages/go';
	import { themeState } from '$lib/state/theme.svelte';
	import 'svelte-highlight/styles/github-dark.css';

	type Props = {
		framework: Framework;
	};

	let { framework }: Props = $props();

	// Projects created for the retired Go SDK instrument with OpenTelemetry now: a span is started from the
	// request's context with the tracer provider the setup guide registers.
	function getCodeExample(fw: Framework): string {
		switch (fw) {
			case 'gin':
				return `// In your Gin handler (otelgin puts the request span in c.Request.Context())
func MyHandler(c *gin.Context) {
    ctx, span := otel.Tracer("my-service").Start(c.Request.Context(), "db.query")
    defer span.End()

    rows, err := db.QueryContext(ctx, "SELECT * FROM users")
    // ...
}`;
			case 'fiber':
				return `// In your Fiber handler (otelfiber puts the request span in c.UserContext())
func MyHandler(c *fiber.Ctx) error {
    ctx, span := otel.Tracer("my-service").Start(c.UserContext(), "db.query")
    defer span.End()

    rows, err := db.QueryContext(ctx, "SELECT * FROM users")
    // ...
    return c.JSON(rows)
}`;
			default:
				return `ctx, span := otel.Tracer("my-service").Start(ctx, "db.load")
// perform your operations here with ctx
span.End()`;
		}
	}

	const isGoFramework = $derived(LEGACY_GO_FRAMEWORKS.includes(framework));
	const codeExample = $derived(getCodeExample(framework));
</script>

<div class="flex flex-col items-center justify-center py-8 text-center">
	<div class="mb-4 rounded-full bg-muted p-3">
		<Code class="h-6 w-6 text-muted-foreground" />
	</div>
	<h3 class="mb-2 text-lg font-semibold">No Spans Recorded</h3>
	<p class="mb-4 max-w-md text-sm text-muted-foreground">
		Spans allow you to track the timing of individual operations within a transaction, such as
		database queries, HTTP calls, or cache operations.
	</p>

	{#if isGoFramework}
		<div class="mb-4 w-full max-w-xl text-left">
			<p class="mb-2 text-xs text-muted-foreground">Example usage:</p>
			<div
				class="overflow-hidden rounded-md text-sm {themeState.isDark ? 'dark-code' : 'light-code'}"
			>
				<Highlight language={go} code={codeExample} />
			</div>
		</div>
	{/if}

	<!-- TODO: After we add the docs we should update this :/ -->
	<!-- <a
		href={docsUrl}
		target="_blank"
		rel="noopener noreferrer"
		class="text-primary inline-flex items-center gap-2 text-sm hover:underline"
	>
		View Documentation
		<ExternalLink class="h-4 w-4" />
	</a> -->
</div>

<style>
	/* Light theme - override dark theme defaults */
	:global(.light-code .hljs) {
		background: #f6f8fa;
		color: #24292e;
	}
	:global(.light-code .hljs-keyword),
	:global(.light-code .hljs-selector-tag) {
		color: #d73a49;
	}
	:global(.light-code .hljs-string),
	:global(.light-code .hljs-attr) {
		color: #032f62;
	}
	:global(.light-code .hljs-function),
	:global(.light-code .hljs-title) {
		color: #6f42c1;
	}
	:global(.light-code .hljs-comment) {
		color: #6a737d;
	}
	:global(.light-code .hljs-built_in) {
		color: #005cc5;
	}

	/* Dark theme - ensure dark styles apply */
	:global(.dark-code .hljs) {
		background: #0d1117;
		color: #c9d1d9;
	}
	:global(.dark-code .hljs-keyword),
	:global(.dark-code .hljs-selector-tag) {
		color: #ff7b72;
	}
	:global(.dark-code .hljs-string),
	:global(.dark-code .hljs-attr) {
		color: #a5d6ff;
	}
	:global(.dark-code .hljs-function),
	:global(.dark-code .hljs-title) {
		color: #d2a8ff;
	}
	:global(.dark-code .hljs-comment) {
		color: #8b949e;
	}
	:global(.dark-code .hljs-built_in) {
		color: #79c0ff;
	}
</style>
