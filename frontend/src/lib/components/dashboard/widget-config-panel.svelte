<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Select from '$lib/components/ui/select';
	import * as Sheet from '$lib/components/ui/sheet';
	import TagFilter from './tag-filter.svelte';
	import { THRESHOLD_COLOR_NAMES, resolveThresholdColor } from './gauge-thresholds';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { Plus, Check, X } from '@lucide/svelte';
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

	function thresholdsFromConfig(
		config: any,
		savedType: string | undefined
	): Array<{ value: string; color: string }> {
		if (Array.isArray(config?.thresholds) && config.thresholds.length > 0) {
			return config.thresholds.map((t: { value: number; color: string }) => ({
				value: String(t.value),
				color: t.color
			}));
		}
		if (savedType === 'single_value') return [];
		return [{ value: '80', color: 'red' }];
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
		widget: { id?: number | string; title: string; widgetType: string; config: any } | null;
		availableMetrics: DiscoveredMetric[];
		error?: string;
		onSave: (data: { title: string; widgetType: string; config: any }) => void;
		onCancel: () => void;
	}>();

	// svelte-ignore state_referenced_locally
	const initialWidget = widget;

	let title = $state(initialWidget?.title ?? '');
	let widgetType = $state(initialWidget?.widgetType ?? 'line_chart');
	let unit = $state(initialWidget?.config?.unit ?? '');
	let unitManuallySet = $state(!!initialWidget?.config?.unit);
	let sources = $state<WidgetSource[]>(
		initialWidget?.config?.sources ?? [{ type: 'metric', name: '', aggregation: 'avg' }]
	);
	let legend = $state<'auto' | 'on' | 'off'>(legendFromConfig(initialWidget?.config));
	let showSparkline = $state(!!initialWidget?.config?.showSparkline);
	let gaugeMin = $state(
		initialWidget?.config?.min != null ? String(initialWidget.config.min) : '0'
	);
	let gaugeMax = $state(
		initialWidget?.config?.max != null ? String(initialWidget.config.max) : '100'
	);
	let baseColor = $state(initialWidget?.config?.baseColor ?? 'green');
	let thresholds = $state<Array<{ value: string; color: string }>>(
		thresholdsFromConfig(initialWidget?.config, initialWidget?.widgetType)
	);

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
			gaugeMin = widget?.config?.min != null ? String(widget.config.min) : '0';
			gaugeMax = widget?.config?.max != null ? String(widget.config.max) : '100';
			baseColor = widget?.config?.baseColor ?? 'green';
			thresholds = thresholdsFromConfig(widget?.config, widget?.widgetType);
		}
	});

	function addThreshold() {
		const values = thresholds.map((t) => parseFloat(t.value)).filter((v) => Number.isFinite(v));
		const nextValue = values.length > 0 ? Math.max(...values) + 10 : 80;
		const usedColors = new Set(thresholds.map((t) => t.color));
		const nextColor =
			['red', 'yellow', 'orange', 'blue', 'green'].find((c) => !usedColors.has(c)) ?? 'red';
		thresholds = [...thresholds, { value: String(nextValue), color: nextColor }];
	}

	function removeThreshold(index: number) {
		thresholds = thresholds.filter((_, i) => i !== index);
	}

	function handleTypeChange(newType: string) {
		const wasValueType = widgetType === 'gauge' || widgetType === 'single_value';
		const isValueType = newType === 'gauge' || newType === 'single_value';
		widgetType = newType;
		if (isValueType && !wasValueType) {
			for (const s of sources) {
				if (s.aggregation === 'avg') s.aggregation = 'last';
			}
		} else if (!isValueType && wasValueType) {
			for (const s of sources) {
				if (s.aggregation === 'last') s.aggregation = 'avg';
			}
		}
	}

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
		if (widgetType === 'gauge') {
			const minValue = parseFloat(gaugeMin);
			const maxValue = parseFloat(gaugeMax);
			config.min = Number.isFinite(minValue) ? minValue : 0;
			config.max = Number.isFinite(maxValue) ? maxValue : 100;
		}
		if (widgetType === 'gauge' || widgetType === 'single_value') {
			if (baseColor !== 'green') config.baseColor = baseColor;
			const steps = thresholds
				.map((t) => ({ value: parseFloat(t.value), color: t.color }))
				.filter((t) => Number.isFinite(t.value))
				.sort((a, b) => a.value - b.value);
			if (steps.length > 0) config.thresholds = steps;
		}
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

<Sheet.Root
	bind:open
	onOpenChange={(v) => {
		if (!v) onCancel();
	}}
>
	<Sheet.Content side="right" class="w-full gap-0 sm:max-w-[520px]">
		<Sheet.Header class="border-b px-6">
			<Sheet.Title>{widget?.id ? 'Edit Widget' : 'New Widget'}</Sheet.Title>
		</Sheet.Header>
		<div class="flex-1 space-y-4 overflow-y-auto px-6 py-4">
			<ErrorAlert {error} />
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
						if (v) handleTypeChange(v);
					}}
				>
					<Select.Trigger>
						{(
							{
								line_chart: 'Line Chart',
								area_chart: 'Area Chart',
								stacked_area: 'Stacked Area',
								bar_chart: 'Bar Chart',
								single_value: 'Single Value',
								gauge: 'Gauge',
								table: 'Table'
							} as Record<string, string>
						)[widgetType] ?? widgetType}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value="line_chart">Line Chart</Select.Item>
						<Select.Item value="area_chart">Area Chart</Select.Item>
						<Select.Item value="stacked_area">Stacked Area</Select.Item>
						<Select.Item value="bar_chart">Bar Chart</Select.Item>
						<Select.Item value="single_value">Single Value</Select.Item>
						<Select.Item value="gauge">Gauge</Select.Item>
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
							{(
								{ auto: 'Auto (when multiple series)', on: 'Always', off: 'Never' } as Record<
									string,
									string
								>
							)[legend]}
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

			{#if widgetType === 'gauge'}
				<div class="grid grid-cols-2 gap-2">
					<div>
						<label class="text-sm font-medium" for="gauge-min">Min</label>
						<Input id="gauge-min" type="number" bind:value={gaugeMin} />
					</div>
					<div>
						<label class="text-sm font-medium" for="gauge-max">Max</label>
						<Input id="gauge-max" type="number" bind:value={gaugeMax} />
					</div>
				</div>
			{/if}

			{#if widgetType === 'gauge' || widgetType === 'single_value'}
				<div>
					<span class="text-sm font-medium">Thresholds</span>
					<p class="mb-2 text-xs text-muted-foreground">
						The value takes the color of the highest threshold it crosses
					</p>
					<div class="space-y-2">
						{#each thresholds as threshold, i (i)}
							<div class="flex items-center gap-2">
								<Input type="number" bind:value={thresholds[i].value} class="h-8 w-28 text-xs" />
								<div class="flex items-center gap-1.5">
									{#each THRESHOLD_COLOR_NAMES as colorName, __index (__index)}
										<button
											type="button"
											class="h-5 w-5 cursor-pointer rounded-full transition-transform {threshold.color ===
											colorName
												? 'ring-2 ring-ring ring-offset-2 ring-offset-background'
												: 'opacity-50 hover:opacity-100'}"
											style="background-color: {resolveThresholdColor(colorName)};"
											aria-label={colorName}
											title={colorName}
											onclick={() => (thresholds[i].color = colorName)}
										></button>
									{/each}
								</div>
								<Button
									variant="ghost"
									size="icon"
									class="h-8 w-8"
									aria-label="Remove threshold"
									onclick={() => removeThreshold(i)}
								>
									<X class="h-4 w-4" />
								</Button>
							</div>
						{/each}
						<div class="flex items-center gap-2">
							<span class="w-28 px-3 text-xs text-muted-foreground">Base</span>
							<div class="flex items-center gap-1.5">
								{#each THRESHOLD_COLOR_NAMES as colorName, __index (__index)}
									<button
										type="button"
										class="h-5 w-5 cursor-pointer rounded-full transition-transform {baseColor ===
										colorName
											? 'ring-2 ring-ring ring-offset-2 ring-offset-background'
											: 'opacity-50 hover:opacity-100'}"
										style="background-color: {resolveThresholdColor(colorName)};"
										aria-label={colorName}
										title={colorName}
										onclick={() => (baseColor = colorName)}
									></button>
								{/each}
							</div>
						</div>
					</div>
					<Button variant="outline" size="sm" class="mt-2" onclick={addThreshold}>
						<Plus class="mr-1 h-3.5 w-3.5" /> Add Threshold
					</Button>
				</div>
			{/if}

			<div>
				<label class="text-sm font-medium" for="widget-unit">Unit (optional)</label>
				<Input
					id="widget-unit"
					bind:value={unit}
					placeholder="Auto-detected from metric"
					oninput={() => {
						unitManuallySet = unit.length > 0;
					}}
				/>
				<p class="mt-1 text-xs text-muted-foreground">%, ms, s, MB, GB, bytes, count, ns</p>
			</div>

			{#each sources as source, i (i)}
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
								{#each availableMetrics as m, __index (__index)}
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
								<Select.Item value="last">last</Select.Item>
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
									{#each getMetricTagKeys(source.name) as key, __index (__index)}
										<Select.Item value={key}>{key}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
					{/if}
				</div>
			{/each}
		</div>
		<Sheet.Footer class="flex-row justify-end border-t px-6">
			<Button variant="outline" onclick={handleClose}>Cancel</Button>
			<Button variant={widget?.id ? 'default' : 'success'} onclick={handleSave}>
				{#if widget?.id}<Check class="mr-2 h-4 w-4" /> Update Widget{:else}<Plus
						class="mr-2 h-4 w-4"
					/> New Widget{/if}
			</Button>
		</Sheet.Footer>
	</Sheet.Content>
</Sheet.Root>
