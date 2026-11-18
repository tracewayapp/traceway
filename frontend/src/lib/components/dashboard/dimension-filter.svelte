<script lang="ts">
	import { api } from '$lib/api';
	import * as Select from '$lib/components/ui/select';

	let {
		label,
		dimension,
		metrics,
		value = '',
		projectId,
		hideWhenEmpty = false,
		onValueChange
	} = $props<{
		label: string;
		/** The dimension key, e.g. `service.version`. */
		dimension: string;
		/** Metrics whose values of the dimension are offered. */
		metrics: string[];
		value?: string;
		projectId?: string | null;
		/** For a dimension a deployment may not have at all (a country needs a lookup source). */
		hideWhenEmpty?: boolean;
		onValueChange?: (value: string) => void;
	}>();

	const ALL = '__all__';
	let options = $state<string[]>([]);
	let loadedFor = '';

	async function load() {
		const key = `${projectId ?? ''}|${dimension}|${metrics.join(',')}`;
		if (!projectId || loadedFor === key) return;
		loadedFor = key;
		const found: Record<string, true> = {};
		await Promise.all(
			metrics.map(async (metric: string) => {
				try {
					const response = await api.get(
						`/metrics/discover/tags?name=${encodeURIComponent(metric)}&key=${encodeURIComponent(dimension)}`,
						{ projectId }
					);
					for (const v of response.values ?? []) found[v] = true;
				} catch {
					// A metric without data offers nothing.
				}
			})
		);
		options = Object.keys(found).sort((a, b) => b.localeCompare(a, undefined, { numeric: true }));
	}

	$effect(() => {
		void projectId;
		void load();
	});
</script>

{#if !hideWhenEmpty || value || options.length > 0}
	<Select.Root
		type="single"
		value={value || ALL}
		onValueChange={(v) => onValueChange?.(v === ALL ? '' : (v ?? ''))}
		onOpenChange={(open) => {
			if (open) {
				loadedFor = '';
				void load();
			}
		}}
	>
		<Select.Trigger class="h-9 max-w-[14rem] gap-1.5 text-sm">
			<span class="text-muted-foreground">{label}</span>
			<span class="truncate">{value || 'All'}</span>
		</Select.Trigger>
		<Select.Content>
			<Select.Item value={ALL}>All</Select.Item>
			{#if value && !options.includes(value)}
				<Select.Item {value}>{value}</Select.Item>
			{/if}
			{#each options as option (option)}
				<Select.Item value={option}>{option}</Select.Item>
			{/each}
		</Select.Content>
	</Select.Root>
{/if}
