<script lang="ts">
	import { ExternalLink, Code } from 'lucide-svelte';
	import type { Framework } from '$lib/state/projects.svelte';
	import Highlight from 'svelte-highlight';
	import go from 'svelte-highlight/languages/go';
	import { themeState } from '$lib/state/theme.svelte';
	import 'svelte-highlight/styles/github-dark.css';

	type Props = {
		framework: Framework;
	};

	let { framework }: Props = $props();

	// TODO: Replace with actual documentation URLs when docs are built
	function getDocsUrl(fw: Framework): string {
		switch (fw) {
			case 'gin':
				return 'https://docs.traceway.io/go/gin/segments';
			case 'fiber':
				return 'https://docs.traceway.io/go/fiber/segments';
			case 'chi':
				return 'https://docs.traceway.io/go/chi/segments';
			case 'fasthttp':
				return 'https://docs.traceway.io/go/fasthttp/segments';
			case 'stdlib':
				return 'https://docs.traceway.io/go/stdlib/segments';
			default:
				return 'https://docs.traceway.io/go/segments';
		}
	}

	function getCodeExample(fw: Framework): string {
		switch (fw) {
			case 'gin':
				return `// In your Gin handler
func MyHandler(c *gin.Context) {
    // Start a segment for database operation
    seg := traceway.StartSegment(c.Request.Context(), "db.query")
    defer seg.End()

    // Your database operation here
    result, err := db.Query("SELECT * FROM users")

    // Another segment for cache
    cacheSeg := traceway.StartSegment(c.Request.Context(), "cache.set")
    cache.Set("users", result)
    cacheSeg.End()
}`;
			case 'fiber':
				return `// In your Fiber handler
func MyHandler(c *fiber.Ctx) error {
    // Start a segment for database operation
    seg := traceway.StartSegment(c.UserContext(), "db.query")
    defer seg.End()

    // Your database operation here
    result, err := db.Query("SELECT * FROM users")

    // Another segment for cache
    cacheSeg := traceway.StartSegment(c.UserContext(), "cache.set")
    cache.Set("users", result)
    cacheSeg.End()

    return c.JSON(result)
}`;
			default:
				return `seg := traceway.StartSegment(ctx, "db.load")
// perform your operations here
seg.End()`;
		}
	}

	const docsUrl = $derived(getDocsUrl(framework));
	const codeExample = $derived(getCodeExample(framework));
</script>

<div class="flex flex-col items-center justify-center py-8 text-center">
	<div class="bg-muted mb-4 rounded-full p-3">
		<Code class="text-muted-foreground h-6 w-6" />
	</div>
	<h3 class="mb-2 text-lg font-semibold">No Segments Recorded</h3>
	<p class="text-muted-foreground mb-4 max-w-md text-sm">
		Segments allow you to track the timing of individual operations within a transaction, such as
		database queries, HTTP calls, or cache operations.
	</p>

	<div class="mb-4 w-full max-w-xl text-left">
		<p class="text-muted-foreground mb-2 text-xs">Example usage:</p>
		<div class="rounded-lg overflow-hidden text-sm {themeState.isDark ? 'dark-code' : 'light-code'}">
			<Highlight language={go} code={codeExample} />
		</div>
	</div>

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
