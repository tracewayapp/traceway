<script lang="ts" module>
	export type SeverityLevel = 'TRACE' | 'DEBUG' | 'INFO' | 'WARN' | 'ERROR' | 'FATAL' | '';

	export function severityFromNumber(n: number | undefined | null): SeverityLevel {
		if (!n || n <= 0) return '';
		if (n <= 4) return 'TRACE';
		if (n <= 8) return 'DEBUG';
		if (n <= 12) return 'INFO';
		if (n <= 16) return 'WARN';
		if (n <= 20) return 'ERROR';
		return 'FATAL';
	}

	export function severityFromText(text: string | undefined | null): SeverityLevel {
		const raw = (text ?? '').toUpperCase();
		if (
			raw === 'TRACE' ||
			raw === 'DEBUG' ||
			raw === 'INFO' ||
			raw === 'WARN' ||
			raw === 'ERROR' ||
			raw === 'FATAL'
		) {
			return raw;
		}
		if (raw === 'WARNING') return 'WARN';
		if (raw === 'CRITICAL') return 'FATAL';
		return '';
	}
</script>

<script lang="ts">
	let {
		severityText,
		severityNumber
	}: {
		severityText?: string;
		severityNumber?: number;
	} = $props();

	const level = $derived<SeverityLevel>(
		severityFromText(severityText) || severityFromNumber(severityNumber)
	);

	const config = $derived(() => {
		switch (level) {
			case 'TRACE':
				return { bg: 'bg-muted', text: 'text-muted-foreground', label: 'TRACE' };
			case 'DEBUG':
				return {
					bg: 'bg-violet-500/15',
					text: 'text-violet-600 dark:text-violet-400',
					label: 'DEBUG'
				};
			case 'INFO':
				return {
					bg: 'bg-sky-500/15',
					text: 'text-sky-600 dark:text-sky-400',
					label: 'INFO'
				};
			case 'WARN':
				return {
					bg: 'bg-yellow-500/15',
					text: 'text-yellow-700 dark:text-yellow-400',
					label: 'WARN'
				};
			case 'ERROR':
				return {
					bg: 'bg-red-500/15',
					text: 'text-red-600 dark:text-red-400',
					label: 'ERROR'
				};
			case 'FATAL':
				return {
					bg: 'bg-red-700/25',
					text: 'text-red-700 dark:text-red-300 font-semibold',
					label: 'FATAL'
				};
			default:
				return null;
		}
	});
</script>

{#if config()}
	<span
		class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium tracking-wide uppercase {config()
			?.bg} {config()?.text}"
	>
		{config()?.label}
	</span>
{/if}
