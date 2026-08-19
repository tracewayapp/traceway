<script lang="ts" module>
	export type MessageSegment =
		| { type: 'text'; text: string }
		| { type: 'param'; name: string; value: string };

	// .NET-style message template placeholders: {Name}, {@Name}, {$Name},
	// optionally with alignment/format specs like {Count,5:N0}.
	const PLACEHOLDER = /\{([@$]?)([A-Za-z_][A-Za-z0-9_.]*)(?:,-?\d+)?(?::[^{}]*)?\}/g;

	// Split a rendered log line into text and parameter segments by resolving
	// template placeholders against the log's attributes. Placeholders without
	// a matching attribute stay as literal text.
	export function segmentMessage(
		line: string,
		attributes: Record<string, string> | null | undefined
	): MessageSegment[] {
		if (!attributes || !line.includes('{')) {
			return [{ type: 'text', text: line }];
		}
		const segments: MessageSegment[] = [];
		let last = 0;
		for (const m of line.matchAll(PLACEHOLDER)) {
			const name = m[2];
			const value = attributes[name] ?? attributes[m[1] + name];
			if (value === undefined) continue;
			if (m.index > last) {
				segments.push({ type: 'text', text: line.slice(last, m.index) });
			}
			segments.push({ type: 'param', name, value });
			last = m.index + m[0].length;
		}
		if (segments.length === 0) {
			return [{ type: 'text', text: line }];
		}
		if (last < line.length) {
			segments.push({ type: 'text', text: line.slice(last) });
		}
		return segments;
	}
</script>

<script lang="ts">
	import * as Tooltip from '$lib/components/ui/tooltip';

	let {
		body,
		attributes
	}: {
		body: string;
		attributes: Record<string, string> | null | undefined;
	} = $props();

	const line = $derived.by(() => {
		if (!body) return '';
		const nl = body.indexOf('\n');
		return nl === -1 ? body : body.slice(0, nl);
	});

	const segments = $derived(segmentMessage(line, attributes));
</script>

{#each segments as seg, i (i)}{#if seg.type === 'text'}{seg.text}{:else}<Tooltip.Root
			><Tooltip.Trigger class="cursor-default align-baseline text-blue-500 dark:text-blue-400"
				>{seg.value}</Tooltip.Trigger
			><Tooltip.Content class="font-mono">{seg.name}</Tooltip.Content></Tooltip.Root
		>{/if}{/each}
