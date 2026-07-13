<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Select from '$lib/components/ui/select';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import TagFilter from './tag-filter.svelte';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { Plus, Check, CircleAlert } from 'lucide-svelte';
	import * as Alert from '$lib/components/ui/alert';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import type { DiscoveredMetric } from '$lib/types/dashboard';

	type WidgetSource = {
		type: 'metric';
		name: string;
		aggregation: string;
		tagFilters?: Record<string, string>;
		groupBy?: string;
		label?: string;
	};

	const CHART_TYPES = ['line_chart', 'area_chart', 'stacked_area'];

	function legendFromConfig(config: any): 'auto' | 'on' | 'off' {
		if (config?.showLegend === true) return 'on';
		if (config?.showLegend === false) return 'off';
		return 'auto';
	}

	let {
		open = $bindable(false),
		widget = null,
		availableMetrics = [],
		error = '',
		onSave,
		onCancel
	} = $props<{
		open: boolean;
		widget: { id?: number; title: string; widgetType: string; config: any } | null;
		availableMetrics: DiscoveredMetric[];
		error?: string;
		onSave: (data: { title: string; widgetType: string; config: any }) => void;
		onCancel: () => void;
	}>();

	let title = $state(widget?.title ?? '');
	let widgetType = $state(widget?.widgetType ?? 'line_chart');
	let unit = $state(widget?.config?.unit ?? '');
	let unitManuallySet = $state(!!widget?.config?.unit);
	let sources = $state<WidgetSource[]>(
		widget?.config?.sources ?? [{ type: 'metric', name: '', aggregation: 'avg' }]
	);
	let legend = $state<'auto' | 'on' | 'off'>(legendFromConfig(widget?.config));
	let showSparkline = $state(!!widget?.config?.showSparkline);

	$effect(() => {
		if (open) {
			title = widget?.title ?? '';
			widgetType = widget?.widgetType ?? 'line_chart';
			unit = widget?.config?.unit ?? '';
			unitManuallySet = !!widget?.config?.unit;
			sources = widget?.config?.sources
				? [...widget.config.sources]
				: [{ type: 'metric', name: '', aggregation: 'avg' }];
			legend = legendFromConfig(widget?.config);
			showSparkline = !!widget?.config?.showSparkline;
		}
	});

	function getMetricTagKeys(metricName: string): string[] {
		const m = availableMetrics.find((m: DiscoveredMetric) => m.name === metricName);
		return m?.tagKeys ?? [];
	}

	function getMetricUnit(metricName: string): string {
		const m = availableMetrics.find((m: DiscoveredMetric) => m.name === metricName);
		return m?.unit ?? '';
	}

	async function loadTagValues(metricName: string, key: string): Promise<string[]> {
		try {
			const response = await api.get(
				`/metrics/discover/tags?name=${encodeURIComponent(metricName)}&key=${encodeURIComponent(key)}`,
				{ projectId: projectsState.currentProjectId ?? undefined }
			);
			return response.values || [];
		} catch {
			return [];
		}
	}

	function handleMetricChange(index: number, value: string) {
		sources[index].name = value;
		if (!unitManuallySet) {
			unit = getMetricUnit(value);
		}
	}

	function handleSave() {
		const validSources = sources
			.filter((s: WidgetSource) => s.name)
			.map((s: WidgetSource) => {
				const { label, ...rest } = s;
				return label?.trim() ? { ...rest, label: label.trim() } : rest;
			});
		const displayTitle = title.trim() || validSources[0]?.name || '';
		const config: Record<string, any> = { sources: validSources };
		if (unit) config.unit = unit;
		if (widget?.config?.colSpan != null) config.colSpan = widget.config.colSpan;
		if (widget?.config?.size != null) config.size = widget.config.size;
		if (CHART_TYPES.includes(widgetType) && legend !== 'auto') config.showLegend = legend === 'on';
		if (widgetType === 'single_value' && showSparkline) config.showSparkline = true;
		onSave({
			title: displayTitle,
			widgetType,
			config
		});
	}

	function handleClose() {
		open = false;
		onCancel();
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content class="max-w-xl" interactOutsideBehavior="close">
		<AlertDialog.Header>
			<AlertDialog.Title>{widget?.id ? 'Edit Widget' : 'New Widget'}</AlertDialog.Title>
		</AlertDialog.Header>
		<ErrorAlert {error} />
		<div class="space-y-4">
			<div>
				<label class="text-sm font-medium" for="widget-title">Title (optional)</label>
				<Input id="widget-title" bind:value={title} placeholder="Defaults to metric name" />
			</div>

			<div>
				<label class="text-sm font-medium">Widget Type</label>
				<Select.Root
					type="single"
					value={widgetType}
					onValueChange={(v) => {
						if (v) widgetType = v;
					}}
				>
					<Select.Trigger>
						{({ line_chart: 'Line Chart', area_chart: 'Area Chart', stacked_area: 'Stacked Area', bar_chart: 'Bar Chart', single_value: 'Single Value', table: 'Table' } as Record<string, string>)[widgetType] ?? widgetType}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="line_chart">Line Chart</Select.Item>
						<Select.Item value="area_chart">Area Chart</Select.Item>
						<Select.Item value="stacked_area">Stacked Area</Select.Item>
						<Select.Item value="bar_chart">Bar Chart</Select.Item>
						<Select.Item value="single_value">Single Value</Select.Item>
						<Select.Item value="table">Table</Select.Item>
					</Select.Content>
				</Select.Root>
			</div>

			{#if CHART_TYPES.includes(widgetType)}
				<div>
					<label class="text-sm font-medium">Legend</label>
					<Select.Root
						type="single"
						value={legend}
						onValueChange={(v) => {
							if (v) legend = v as 'auto' | 'on' | 'off';
						}}
					>
						<Select.Trigger>
							{({ auto: 'Auto (when multiple series)', on: 'Always', off: 'Never' } as Record<string, string>)[legend]}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="auto">Auto (when multiple series)</Select.Item>
							<Select.Item value="on">Always</Select.Item>
							<Select.Item value="off">Never</Select.Item>
						</Select.Content>
					</Select.Root>
				</div>
			{/if}

			{#if widgetType === 'single_value'}
				<div class="flex items-center gap-2">
					<Checkbox
						checked={showSparkline}
						onCheckedChange={(c) => (showSparkline = c === true)}
						aria-label="Show sparkline"
					/>
					<button
						type="button"
						class="cursor-pointer text-sm font-medium"
						onclick={() => (showSparkline = !showSparkline)}
					>
						Show sparkline
					</button>
				</div>
			{/if}

			<div>
				<label class="text-sm font-medium" for="widget-unit">Unit (optional)</label>
				<Input
					id="widget-unit"
					bind:value={unit}
					placeholder="Auto-detected from metric"
					oninput={() => { unitManuallySet = unit.length > 0; }}
				/>
				<p class="text-xs text-muted-foreground mt-1">%, ms, s, MB, GB, bytes, count, ns</p>
			</div>

			{#each sources as source, i}
				<div class="space-y-2 rounded-md border p-3">
					<div class="flex items-center gap-2">
						<Select.Root
							type="single"
							value={source.name}
							onValueChange={(v) => {
								if (v) handleMetricChange(i, v);
							}}
						>
							<Select.Trigger class="flex-1">{source.name || 'Select metric'}</Select.Trigger>
							<Select.Content>
								{#each availableMetrics as m}
									<Select.Item value={m.name}>{m.name}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>

						<Select.Root
							type="single"
							value={source.aggregation}
							onValueChange={(v) => {
								if (v) sources[i].aggregation = v;
							}}
						>
							<Select.Trigger class="w-20">{source.aggregation}</Select.Trigger>
							<Select.Content>
								<Select.Item value="avg">avg</Select.Item>
								<Select.Item value="min">min</Select.Item>
								<Select.Item value="max">max</Select.Item>
								<Select.Item value="sum">sum</Select.Item>
								<Select.Item value="count">count</Select.Item>
							</Select.Content>
						</Select.Root>
					</div>

					<Input
						bind:value={sources[i].label}
						placeholder="Label (optional, defaults to metric name)"
						class="h-8 text-xs"
					/>

					{#if source.name && getMetricTagKeys(source.name).length > 0}
						<div class="space-y-1">
							<span class="text-xs text-muted-foreground">Tag Filters</span>
							<TagFilter
								tagKeys={getMetricTagKeys(source.name)}
								activeFilters={source.tagFilters ?? {}}
								onFilterChange={(filters) => {
									sources[i].tagFilters = Object.keys(filters).length > 0 ? filters : undefined;
								}}
								onLoadTagValues={(key) => loadTagValues(source.name, key)}
							/>
						</div>

						<div>
							<span class="text-xs text-muted-foreground">Group By</span>
							<Select.Root
								type="single"
								value={source.groupBy ?? ''}
								onValueChange={(v) => {
									sources[i].groupBy = v || undefined;
								}}
							>
								<Select.Trigger class="h-7 w-[160px] text-xs"
									>{source.groupBy || 'None'}</Select.Trigger
								>
								<Select.Content>
									<Select.Item value="">None</Select.Item>
									{#each getMetricTagKeys(source.name) as key}
										<Select.Item value={key}>{key}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
					{/if}
				</div>
			{/each}
		</div>
		<AlertDialog.Footer>
			<Button variant="outline" onclick={handleClose}>Cancel</Button>
			<Button variant={widget?.id ? 'default' : 'success'} onclick={handleSave}>
				{#if widget?.id}<Check class="mr-1 h-4 w-4" /> Update Widget{:else}<Plus class="mr-1 h-4 w-4" /> New Widget{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
