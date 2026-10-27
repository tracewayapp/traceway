<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '$lib/api';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { WarningCallout } from '$lib/components/ui/warning-callout';
	import KeyValueRows from '$lib/components/synthetics/key-value-rows.svelte';
	import TagFilter from '$lib/components/dashboard/tag-filter.svelte';
	import { projectsState, isFrontendFramework } from '$lib/state/projects.svelte';
	import {
		BACKEND_SLI_KINDS,
		FRONTEND_SLI_KINDS,
		SLI_KIND_LABELS,
		SLO_WINDOW_OPTIONS,
		type GoodCondition,
		type MetricAggregation,
		type MetricOperator,
		type SliConfig,
		type SliKind,
		type SLO,
		type SloPreview,
		type SpanSliAttribute
	} from '$lib/state/slos.svelte';
	import type { DiscoveredMetric } from '$lib/types/dashboard';
	import { SPAN_KIND_OPTIONS } from '$lib/utils/span-fields';
	import { parseSpanAttributeFilter, type SpanAttributeFilter } from '$lib/utils/span-explorer';
	import { getErrorMessage } from '$lib/utils/errors';
	import { formatSloPercent } from '$lib/utils/slo-math';
	import { Check, FlaskConical, Plus } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	interface Props {
		open: boolean;
		slo: SLO | null;
		onSaved: () => void;
	}

	let { open = $bindable(), slo, onSaved }: Props = $props();

	const isEditing = $derived(slo !== null);
	let loading = $state(false);
	let error = $state('');

	let name = $state('');
	let description = $state('');
	let sliKind = $state<SliKind>('endpoint');
	let targetPercent = $state('99.9');
	let windowDays = $state('30');
	let enabled = $state(true);

	let endpointName = $state('');
	let endpointGood = $state<GoodCondition>('availability');
	let endpointLatencyMs = $state('500');

	let spanService = $state('');
	let spanName = $state('');
	let spanKind = $state('');
	let spanAttrRows = $state<{ key: string; value: string }[]>([]);
	let spanGood = $state<GoodCondition>('availability');
	let spanLatencyMs = $state('500');

	let taskName = $state('');

	let checkIds = $state<string[]>([]);

	let metricName = $state('');
	let metricTags = $state<Record<string, string>>({});
	let metricAggregation = $state<MetricAggregation>('avg');
	let metricOperator = $state<MetricOperator>('lt');
	let metricThreshold = $state('');
	let sliceMinutes = $state('5');
	let missingIsBad = $state(false);

	let sessionAppVersion = $state('');
	let includeMessages = $state(false);

	let endpointOptions = $state<string[]>([]);
	let taskOptions = $state<string[]>([]);
	let serviceOptions = $state<string[]>([]);
	let checks = $state<{ id: number; name: string; checkType: string }[]>([]);
	let metrics = $state<DiscoveredMetric[]>([]);
	let loadedPickers = $state<Record<string, boolean>>({});

	let previewing = $state(false);
	let preview = $state<SloPreview | null>(null);
	let previewError = $state('');

	let originalDefinition = $state('');

	const isFrontendProject = $derived(
		!!projectsState.currentProject && isFrontendFramework(projectsState.currentProject.framework)
	);
	const defaultKind = $derived<SliKind>(isFrontendProject ? 'session' : 'endpoint');
	const kindOptions = $derived.by(() => {
		const kinds = isFrontendProject ? FRONTEND_SLI_KINDS : BACKEND_SLI_KINDS;
		const list = slo && !kinds.includes(slo.sliKind) ? [...kinds, slo.sliKind] : kinds;
		return list.map((kind) => ({ value: kind, label: SLI_KIND_LABELS[kind] }));
	});
	const kindHelp: Record<SliKind, string> = {
		endpoint: 'Requests of one endpoint, or of every endpoint in the project.',
		span: 'Any span matching the same filters the span explorer offers, so a queue consumer or an outbound call can have its own SLO.',
		task: 'Background task runs. A run is bad when an exception was recorded on its own span.',
		monitor: 'Probes of the selected monitors. Good means up; missed probes do not count.',
		metric: 'Time slices of a metric. A slice is good when its aggregate satisfies the comparison.',
		session:
			'Sessions reported by the browser and mobile SDKs. A session is bad when an exception was recorded in it; captured messages only count if you say so.'
	};
	const goodOptions = [
		{ value: 'availability', label: 'Status below 500' },
		{ value: 'latency', label: 'Faster than a threshold' }
	];
	const spanGoodOptions = [
		{ value: 'availability', label: 'Span status is not error' },
		{ value: 'latency', label: 'Faster than a threshold' }
	];
	const spanKindOptions = [{ value: '', label: 'Any kind' }, ...SPAN_KIND_OPTIONS];
	const sliceOptions = [
		{ value: '1', label: '1 minute' },
		{ value: '5', label: '5 minutes' },
		{ value: '15', label: '15 minutes' },
		{ value: '60', label: '1 hour' }
	];
	const aggregationOptions = [
		{ value: 'avg', label: 'Average' },
		{ value: 'min', label: 'Minimum' },
		{ value: 'max', label: 'Maximum' },
		{ value: 'sum', label: 'Sum' },
		{ value: 'last', label: 'Last value' }
	];
	const operatorOptions = [
		{ value: 'lt', label: 'below' },
		{ value: 'lte', label: 'at most' },
		{ value: 'gt', label: 'above' },
		{ value: 'gte', label: 'at least' }
	];

	function labelOf(options: { value: string; label: string }[], value: string, fallback: string) {
		return options.find((option) => option.value === value)?.label ?? fallback;
	}

	function resetForm() {
		name = '';
		description = '';
		sliKind = defaultKind;
		targetPercent = '99.9';
		windowDays = '30';
		enabled = true;
		endpointName = '';
		endpointGood = 'availability';
		endpointLatencyMs = '500';
		spanService = '';
		spanName = '';
		spanKind = '';
		spanAttrRows = [];
		spanGood = 'availability';
		spanLatencyMs = '500';
		taskName = '';
		checkIds = [];
		metricName = '';
		metricTags = {};
		metricAggregation = 'avg';
		metricOperator = 'lt';
		metricThreshold = '';
		sliceMinutes = '5';
		missingIsBad = false;
		sessionAppVersion = '';
		includeMessages = false;
		preview = null;
		previewError = '';
		error = '';
		originalDefinition = '';
	}

	function populateFromSlo(s: SLO) {
		resetForm();
		name = s.name;
		description = s.description;
		sliKind = s.sliKind;
		targetPercent = String(s.targetPercent);
		windowDays = String(s.windowDays);
		enabled = s.enabled;
		const cfg = s.sliConfig as unknown as Record<string, unknown>;
		switch (s.sliKind) {
			case 'endpoint':
				endpointName = (cfg.endpoint as string) ?? '';
				endpointGood = (cfg.good as GoodCondition) ?? 'availability';
				endpointLatencyMs = String((cfg.latencyThresholdMs as number) || 500);
				break;
			case 'span':
				spanService = (cfg.serviceName as string) ?? '';
				spanName = (cfg.name as string) ?? '';
				spanKind = cfg.kind === null || cfg.kind === undefined ? '' : String(cfg.kind);
				spanAttrRows = ((cfg.attributes as SpanSliAttribute[]) ?? []).map((a) => ({ ...a }));
				spanGood = (cfg.good as GoodCondition) ?? 'availability';
				spanLatencyMs = String((cfg.latencyThresholdMs as number) || 500);
				break;
			case 'task':
				taskName = (cfg.taskName as string) ?? '';
				break;
			case 'monitor':
				checkIds = ((cfg.checkIds as number[]) ?? []).map(String);
				break;
			case 'metric':
				metricName = (cfg.metricName as string) ?? '';
				metricTags = { ...((cfg.tags as Record<string, string>) ?? {}) };
				metricAggregation = (cfg.aggregation as MetricAggregation) ?? 'avg';
				metricOperator = (cfg.operator as MetricOperator) ?? 'lt';
				metricThreshold = String(cfg.threshold ?? '');
				sliceMinutes = String(s.bucketMinutes || 5);
				missingIsBad = !!cfg.missingIsBad;
				break;
			case 'session':
				sessionAppVersion = (cfg.appVersion as string) ?? '';
				includeMessages = !!cfg.includeMessages;
				break;
		}
		originalDefinition = currentDefinition() ?? '';
	}

	$effect(() => {
		const current = slo;
		if (open && current) {
			untrack(() => populateFromSlo(current));
		} else if (open && !current) {
			untrack(resetForm);
		}
		if (!open) {
			loadedPickers = {};
		}
	});

	$effect(() => {
		if (!open) return;
		loadPickers(sliKind);
	});

	$effect(() => {
		void currentDefinition();
		preview = null;
		previewError = '';
	});

	function handleOpenChange(value: boolean) {
		open = value;
		if (!value) {
			error = '';
		}
	}

	function rangeParams(days: number): { fromDate: string; toDate: string } {
		const now = new Date();
		return {
			fromDate: new Date(now.getTime() - days * 24 * 60 * 60 * 1000).toISOString(),
			toDate: now.toISOString()
		};
	}

	const projectQuery = () => ({ projectId: projectsState.currentProjectId ?? undefined });

	async function loadPickers(kind: SliKind) {
		if (loadedPickers[kind]) return;
		loadedPickers = { ...loadedPickers, [kind]: true };
		try {
			switch (kind) {
				case 'endpoint': {
					const response = await api.post(
						'/endpoints/grouped',
						{
							...rangeParams(7),
							orderBy: 'count',
							sortDirection: 'desc',
							search: '',
							rootFilter: '',
							pagination: { page: 1, pageSize: 100 }
						},
						projectQuery()
					);
					endpointOptions = (response.data || []).map((row: { endpoint: string }) => row.endpoint);
					break;
				}
				case 'task': {
					const response = await api.post(
						'/tasks/grouped',
						{
							...rangeParams(7),
							orderBy: 'count',
							sortDirection: 'desc',
							search: '',
							rootFilter: '',
							pagination: { page: 1, pageSize: 100 }
						},
						projectQuery()
					);
					taskOptions = (response.data || []).map((row: { taskName: string }) => row.taskName);
					break;
				}
				case 'span': {
					const response = await api.post(
						'/spans/search',
						{ ...rangeParams(1), orderBy: 'start_time desc', pagination: { page: 1, pageSize: 1 } },
						projectQuery()
					);
					serviceOptions = response.services || [];
					break;
				}
				case 'monitor': {
					const response = await api.get('/synthetics/checks', projectQuery());
					checks = response.checks || [];
					break;
				}
				case 'metric': {
					const { fromDate, toDate } = rangeParams(7);
					const response = await api.get(
						`/metrics/discover?from=${fromDate}&to=${toDate}`,
						projectQuery()
					);
					metrics = response.metrics || [];
					break;
				}
			}
		} catch (e) {
			console.error('Failed to load SLO picker options:', e);
		}
	}

	async function loadTagValues(key: string): Promise<string[]> {
		try {
			const response = await api.get(
				`/metrics/discover/tags?name=${encodeURIComponent(metricName)}&key=${encodeURIComponent(key)}`,
				projectQuery()
			);
			return response.values || [];
		} catch {
			return [];
		}
	}

	const metricTagKeys = $derived(metrics.find((m) => m.name === metricName)?.tagKeys ?? []);

	const serviceSelectOptions = $derived([
		{ value: '', label: 'Any service' },
		...[...new Set([...serviceOptions, spanService].filter((s) => !!s))].map((s) => ({
			value: s,
			label: s
		}))
	]);

	const checksLabel = $derived.by(() => {
		if (checkIds.length === 0) return 'Select monitors';
		const names = checks.filter((c) => checkIds.includes(String(c.id))).map((c) => c.name);
		if (names.length === 0) return `${checkIds.length} selected`;
		return names.join(', ');
	});

	function buildSpanAttributes(): SpanAttributeFilter[] {
		let attributes: SpanAttributeFilter[] = [];
		for (const row of spanAttrRows) {
			if (!row.key.trim() && !row.value.trim()) continue;
			const parsed = parseSpanAttributeFilter(row.key, row.value, attributes);
			if (typeof parsed === 'string') {
				throw new Error(parsed);
			}
			attributes = [...attributes, parsed];
		}
		return attributes;
	}

	function buildSliConfig(): SliConfig {
		switch (sliKind) {
			case 'endpoint':
				return {
					endpoint: endpointName.trim(),
					good: endpointGood,
					latencyThresholdMs: Number(endpointLatencyMs) || 0
				};
			case 'span':
				return {
					serviceName: spanService,
					name: spanName.trim(),
					kind: spanKind === '' ? null : Number(spanKind),
					attributes: buildSpanAttributes(),
					good: spanGood,
					latencyThresholdMs: Number(spanLatencyMs) || 0
				};
			case 'task':
				return { taskName: taskName.trim() };
			case 'monitor':
				return { checkIds: checkIds.map(Number) };
			case 'metric':
				return {
					metricName: metricName.trim(),
					tags: metricTags,
					aggregation: metricAggregation,
					operator: metricOperator,
					threshold: String(metricThreshold ?? '').trim() === '' ? null : Number(metricThreshold),
					missingIsBad
				};
			case 'session':
				return { appVersion: sessionAppVersion.trim(), includeMessages };
		}
	}

	function currentDefinition(): string | null {
		try {
			return JSON.stringify({
				sliKind,
				sliConfig: buildSliConfig(),
				bucketMinutes: sliKind === 'metric' ? Number(sliceMinutes) : 1
			});
		} catch {
			return null;
		}
	}

	const definitionChanged = $derived.by(() => {
		if (!isEditing || !originalDefinition) return false;
		const current = currentDefinition();
		return current !== null && current !== originalDefinition;
	});

	async function runPreview() {
		previewing = true;
		previewError = '';
		preview = null;
		const definition = currentDefinition();
		try {
			const result = await api.post(
				'/slos/preview',
				{
					sliKind,
					sliConfig: buildSliConfig(),
					bucketMinutes: sliKind === 'metric' ? Number(sliceMinutes) : 1
				},
				projectQuery()
			);
			if (currentDefinition() === definition) preview = result;
		} catch (e) {
			if (currentDefinition() === definition) previewError = getErrorMessage(e) || 'Preview failed';
		} finally {
			previewing = false;
		}
	}

	async function handleSubmit() {
		loading = true;
		error = '';
		try {
			const payload = {
				name: name.trim(),
				description: description.trim(),
				sliKind,
				sliConfig: buildSliConfig(),
				targetPercent: Number(targetPercent),
				windowDays: Number(windowDays),
				bucketMinutes: sliKind === 'metric' ? Number(sliceMinutes) : 1,
				enabled
			};
			if (isEditing && slo) {
				await api.put(`/slos/${slo.id}`, payload, projectQuery());
				toast.success('Successfully updated the SLO', { position: 'top-center' });
			} else {
				await api.post('/slos', payload, projectQuery());
				toast.success('Successfully created the SLO', { position: 'top-center' });
			}
			open = false;
			onSaved();
		} catch (e) {
			error = getErrorMessage(e) || 'Failed to save the SLO';
		} finally {
			loading = false;
		}
	}
</script>

<Sheet.Root {open} onOpenChange={handleOpenChange}>
	<Sheet.Content
		side="right"
		class="flex w-full flex-col gap-0 overflow-y-auto sm:w-[760px] sm:max-w-[min(760px,100vw)]"
	>
		<Sheet.Header class="border-b px-6 pb-4">
			<Sheet.Title>{isEditing ? 'Edit SLO' : 'New SLO'}</Sheet.Title>
			<Sheet.Description>
				{isEditing
					? 'Update the objective. Changing what the indicator measures recomputes its history.'
					: 'Pick what counts as a good event, the target share, and the rolling window to hold it over.'}
			</Sheet.Description>
		</Sheet.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			class="flex-1 space-y-5 px-6 py-5"
		>
			<ErrorAlert {error} />

			<div class="grid gap-4 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="slo-name">Name</Label>
					<Input
						id="slo-name"
						bind:value={name}
						placeholder="Checkout availability"
						disabled={loading}
					/>
				</div>
				<div class="space-y-2">
					<Label for="slo-description">Description</Label>
					<Input
						id="slo-description"
						bind:value={description}
						placeholder="What this objective protects"
						disabled={loading}
					/>
				</div>
			</div>

			<div class="grid gap-4 sm:grid-cols-3">
				<div class="space-y-2">
					<Label>Indicator</Label>
					<Select.Root type="single" bind:value={sliKind} disabled={loading}>
						<Select.Trigger class="w-full">{SLI_KIND_LABELS[sliKind]}</Select.Trigger>
						<Select.Content>
							{#each kindOptions as option (option.value)}
								<Select.Item value={option.value}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="space-y-2">
					<Label for="slo-target">Target (%)</Label>
					<Input
						id="slo-target"
						type="number"
						step="0.01"
						min="0"
						max="100"
						bind:value={targetPercent}
						disabled={loading}
					/>
				</div>
				<div class="space-y-2">
					<Label>Window</Label>
					<Select.Root type="single" bind:value={windowDays} disabled={loading}>
						<Select.Trigger class="w-full">
							{labelOf(SLO_WINDOW_OPTIONS, windowDays, 'Select window')}
						</Select.Trigger>
						<Select.Content>
							{#each SLO_WINDOW_OPTIONS as option (option.value)}
								<Select.Item value={option.value}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>

			<div class="space-y-4 rounded-md border p-4">
				<p class="text-xs text-muted-foreground">{kindHelp[sliKind]}</p>

				{#if sliKind === 'endpoint'}
					<div class="space-y-2">
						<Label for="slo-endpoint">Endpoint</Label>
						<Input
							id="slo-endpoint"
							list="slo-endpoint-options"
							bind:value={endpointName}
							placeholder="Leave empty for all endpoints, or GET /api/checkout"
							disabled={loading}
						/>
						<datalist id="slo-endpoint-options">
							{#each endpointOptions as option (option)}
								<option value={option}></option>
							{/each}
						</datalist>
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<div class="space-y-2">
							<Label>Good when</Label>
							<Select.Root type="single" bind:value={endpointGood} disabled={loading}>
								<Select.Trigger class="w-full"
									>{labelOf(goodOptions, endpointGood, 'Select')}</Select.Trigger
								>
								<Select.Content>
									{#each goodOptions as option (option.value)}
										<Select.Item value={option.value}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						{#if endpointGood === 'latency'}
							<div class="space-y-2">
								<Label for="slo-endpoint-latency">Threshold (ms)</Label>
								<Input
									id="slo-endpoint-latency"
									type="number"
									min="1"
									bind:value={endpointLatencyMs}
									disabled={loading}
								/>
							</div>
						{/if}
					</div>
				{:else if sliKind === 'span'}
					<div class="grid gap-4 sm:grid-cols-3">
						<div class="space-y-2">
							<Label>Service</Label>
							<Select.Root type="single" bind:value={spanService} disabled={loading}>
								<Select.Trigger class="w-full">
									<span class="truncate"
										>{labelOf(serviceSelectOptions, spanService, 'Any service')}</span
									>
								</Select.Trigger>
								<Select.Content class="max-h-64 overflow-y-auto">
									{#each serviceSelectOptions as option (option.value)}
										<Select.Item value={option.value}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						<div class="space-y-2">
							<Label for="slo-span-name">Name contains</Label>
							<Input
								id="slo-span-name"
								bind:value={spanName}
								placeholder="POST /checkout"
								disabled={loading}
							/>
						</div>
						<div class="space-y-2">
							<Label>Kind</Label>
							<Select.Root type="single" bind:value={spanKind} disabled={loading}>
								<Select.Trigger class="w-full"
									>{labelOf(spanKindOptions, spanKind, 'Any kind')}</Select.Trigger
								>
								<Select.Content>
									{#each spanKindOptions as option (option.value)}
										<Select.Item value={option.value}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
					</div>
					<div class="space-y-2">
						<Label>Attribute filters</Label>
						<KeyValueRows
							bind:rows={spanAttrRows}
							keyPlaceholder="http.route"
							valuePlaceholder="/checkout"
							addLabel="Add attribute"
							disabled={loading}
						/>
						<p class="text-xs text-muted-foreground">
							Exact matches on span attributes, the OpenTelemetry names. Up to 10 filters.
						</p>
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<div class="space-y-2">
							<Label>Good when</Label>
							<Select.Root type="single" bind:value={spanGood} disabled={loading}>
								<Select.Trigger class="w-full"
									>{labelOf(spanGoodOptions, spanGood, 'Select')}</Select.Trigger
								>
								<Select.Content>
									{#each spanGoodOptions as option (option.value)}
										<Select.Item value={option.value}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						{#if spanGood === 'latency'}
							<div class="space-y-2">
								<Label for="slo-span-latency">Threshold (ms)</Label>
								<Input
									id="slo-span-latency"
									type="number"
									min="1"
									bind:value={spanLatencyMs}
									disabled={loading}
								/>
							</div>
						{/if}
					</div>
				{:else if sliKind === 'task'}
					<div class="space-y-2">
						<Label for="slo-task">Task</Label>
						<Input
							id="slo-task"
							list="slo-task-options"
							bind:value={taskName}
							placeholder="Leave empty for all tasks, or sync_users"
							disabled={loading}
						/>
						<datalist id="slo-task-options">
							{#each taskOptions as option (option)}
								<option value={option}></option>
							{/each}
						</datalist>
					</div>
				{:else if sliKind === 'monitor'}
					<div class="space-y-2">
						<Label>Monitors</Label>
						<Select.Root type="multiple" bind:value={checkIds} disabled={loading}>
							<Select.Trigger class="w-full">
								<span class="truncate">{checksLabel}</span>
							</Select.Trigger>
							<Select.Content class="max-h-64 overflow-y-auto">
								{#each checks as check (check.id)}
									<Select.Item value={String(check.id)} label={check.name}>
										{check.name}
										<span class="ml-2 text-xs text-muted-foreground uppercase"
											>{check.checkType}</span
										>
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
						{#if checks.length === 0}
							<p class="text-xs text-muted-foreground">No monitors in this project yet.</p>
						{/if}
					</div>
				{:else if sliKind === 'metric'}
					<div class="space-y-2">
						<Label for="slo-metric">Metric</Label>
						<Input
							id="slo-metric"
							list="slo-metric-options"
							bind:value={metricName}
							placeholder="system.cpu.utilization"
							disabled={loading}
						/>
						<datalist id="slo-metric-options">
							{#each metrics as metric (metric.name)}
								<option value={metric.name}></option>
							{/each}
						</datalist>
					</div>
					{#if metricTagKeys.length > 0}
						<div class="space-y-2">
							<Label>Tag filters</Label>
							<TagFilter
								tagKeys={metricTagKeys}
								activeFilters={metricTags}
								onFilterChange={(filters) => (metricTags = filters)}
								onLoadTagValues={loadTagValues}
							/>
						</div>
					{/if}
					<div class="grid gap-4 sm:grid-cols-3">
						<div class="space-y-2">
							<Label>Aggregate per slice</Label>
							<Select.Root type="single" bind:value={metricAggregation} disabled={loading}>
								<Select.Trigger class="w-full">
									{labelOf(aggregationOptions, metricAggregation, 'Average')}
								</Select.Trigger>
								<Select.Content>
									{#each aggregationOptions as option (option.value)}
										<Select.Item value={option.value}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						<div class="space-y-2">
							<Label>Good when value is</Label>
							<Select.Root type="single" bind:value={metricOperator} disabled={loading}>
								<Select.Trigger class="w-full"
									>{labelOf(operatorOptions, metricOperator, 'below')}</Select.Trigger
								>
								<Select.Content>
									{#each operatorOptions as option (option.value)}
										<Select.Item value={option.value}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						<div class="space-y-2">
							<Label for="slo-metric-threshold">Threshold</Label>
							<Input
								id="slo-metric-threshold"
								type="number"
								step="any"
								bind:value={metricThreshold}
								placeholder="90"
								disabled={loading}
							/>
						</div>
					</div>
					<div class="grid gap-4 sm:grid-cols-2">
						<div class="space-y-2">
							<Label>Slice length</Label>
							<Select.Root type="single" bind:value={sliceMinutes} disabled={loading}>
								<Select.Trigger class="w-full"
									>{labelOf(sliceOptions, sliceMinutes, '5 minutes')}</Select.Trigger
								>
								<Select.Content>
									{#each sliceOptions as option (option.value)}
										<Select.Item value={option.value}>{option.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						<label class="flex cursor-pointer items-center gap-2 pt-7 text-sm">
							<Checkbox
								checked={missingIsBad}
								onCheckedChange={(checked) => (missingIsBad = checked === true)}
								disabled={loading}
							/>
							Slices without data count as bad
						</label>
					</div>
				{:else if sliKind === 'session'}
					<div class="grid gap-4 sm:grid-cols-2">
						<div class="space-y-2">
							<Label for="slo-app-version">App version</Label>
							<Input
								id="slo-app-version"
								bind:value={sessionAppVersion}
								placeholder="Leave empty for every version, or 1.4.2"
								disabled={loading}
							/>
						</div>
						<label class="flex cursor-pointer items-center gap-2 pt-7 text-sm">
							<Checkbox
								checked={includeMessages}
								onCheckedChange={(checked) => (includeMessages = checked === true)}
								disabled={loading}
							/>
							Captured messages count as failures
						</label>
					</div>
				{/if}

				<div class="flex flex-wrap items-center gap-3 border-t pt-3">
					<Button
						type="button"
						variant="outline"
						size="sm"
						onclick={runPreview}
						disabled={loading || previewing}
					>
						<FlaskConical class="mr-2 h-4 w-4" />
						{previewing ? 'Computing...' : 'Preview last 24h'}
					</Button>
					{#if previewError}
						<span class="text-sm text-red-600 dark:text-red-400">{previewError}</span>
					{:else if preview}
						<span class="font-mono text-sm tabular-nums">
							{#if preview.sli === null}
								Last 24h: no events matched
							{:else}
								Last 24h: {formatSloPercent(preview.sli, Number(targetPercent) || 99.9)}
								<span class="text-muted-foreground">
									({preview.good.toLocaleString()} / {preview.total.toLocaleString()})
								</span>
							{/if}
						</span>
					{/if}
				</div>
			</div>

			{#if definitionChanged}
				<WarningCallout title="History will be recomputed">
					Changing what the indicator measures recalculates every bucket from your retained
					telemetry. The current SLI and budget reset until the backfill finishes.
				</WarningCallout>
			{/if}

			<label class="flex cursor-pointer items-center gap-2 text-sm">
				<Checkbox
					checked={enabled}
					onCheckedChange={(checked) => (enabled = checked === true)}
					disabled={loading}
				/>
				Enabled
			</label>
		</form>

		<div class="flex justify-end gap-2 border-t px-6 py-4">
			<Button variant="outline" onclick={() => handleOpenChange(false)} disabled={loading}
				>Cancel</Button
			>
			<Button variant={isEditing ? 'default' : 'success'} onclick={handleSubmit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{loading ? 'Updating...' : 'Update SLO'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating...' : 'New SLO'}
				{/if}
			</Button>
		</div>
	</Sheet.Content>
</Sheet.Root>
