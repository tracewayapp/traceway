<script lang="ts">
	import SdkVersionWarning from './sdk-version-warning.svelte';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import * as Tabs from '$lib/components/ui/tabs';
	import { CodeEditor } from '$lib/components/ui/code-editor';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Check, Plus, X } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { isBrowserFramework, projectsState } from '$lib/state/projects.svelte';
	import LogTriggerBuilder from './log-trigger-builder.svelte';
	import SequenceTriggerBuilder from './sequence-trigger-builder.svelte';
	import {
		BROWSER_ONLY_SIGNALS,
		CONDITION_OPS,
		SIGNALS,
		SUGGESTED_KEYS,
		TRIGGER_TEMPLATES,
		fitsForm,
		withTrigger,
		buildRuleConditions,
		conditionText,
		isNumericOp,
		optionalNumber,
		signalsFor,
		type CaptureMode,
		type ConditionRow,
		type Mode,
		type SdkConfigRule,
		type SettingsDocument
	} from './sdk-config';

	interface Props {
		open: boolean;
		rule: SdkConfigRule | null;
		/** Trigger ids the project's other rules use; a trigger added here gets an id none of them has. */
		otherTriggerIds?: string[];
		onSaved: () => void;
	}

	let { open = $bindable(), rule, otherTriggerIds = [], onSaved }: Props = $props();
	let flutter = $derived(projectsState.currentProject?.framework === 'flutter');
	// Recorded sequences match clicks, hovers and navigation, which the browser and Flutter SDKs report.
	let recordsSequences = $derived(
		!!projectsState.currentProject &&
			(isBrowserFramework(projectsState.currentProject.framework) || flutter)
	);
	let triggerTemplates = $derived(TRIGGER_TEMPLATES.filter((t) => !flutter || !t.browserOnly));

	type Choice = 'inherit' | Mode;

	let name = $state('');
	let formTriggers = $state<unknown[]>([]);
	let enabled = $state(true);
	let rows = $state<ConditionRow[]>([]);
	let rolloutPercent = $state<number | null | undefined>(undefined);
	let rolloutBy = $state('');
	let baselinePercent = $state<number | null | undefined>(undefined);
	let editAsJson = $state(false);
	type RuleTab = 'applies' | 'triggers' | 'signals' | 'json';
	let tab = $state<RuleTab>('applies');
	let signalChoices = $state<Record<string, Choice>>({});
	let shownSignals = $state(SIGNALS);
	let replayChoice = $state<Choice>('inherit');
	let captureModeChoice = $state<'inherit' | CaptureMode>('inherit');
	let showCaptureMode = $state(false);
	let sessionsChoice = $state<'inherit' | 'on' | 'off'>('inherit');
	let collectionChoice = $state<'inherit' | 'on' | 'off'>('inherit');
	let metricsChoice = $state<'inherit' | 'on' | 'off'>('inherit');
	let metricsSamplePercent = $state<number | null | undefined>(undefined);
	let settingsJson = $state('');
	let saving = $state(false);
	let dialogError = $state('');

	const choiceLabels: Record<Choice, string> = {
		inherit: 'Inherit',
		off: 'Off',
		buffer: 'Buffer',
		stream: 'Stream'
	};
	const toggleLabels = { inherit: 'Inherit', on: 'On', off: 'Off' };
	const captureModeLabels: Record<'inherit' | CaptureMode, string> = {
		inherit: 'Inherit',
		video: 'Video',
		wireframe: 'Wireframe'
	};

	$effect(() => {
		if (!open) return;
		dialogError = '';
		name = rule?.name ?? '';
		enabled = rule?.enabled ?? true;
		rows = (rule?.conditions.all ?? []).map((c) => ({
			key: c.key,
			op: c.op,
			value: conditionText(c),
			original: c
		}));
		rolloutPercent = rule?.conditions.rollout?.percent;
		rolloutBy = rule?.conditions.rollout?.by ?? '';
		const settings = rule?.settings ?? { schemaVersion: 1 };
		loadForm(settings);
		settingsJson = JSON.stringify(settings, null, 2);
		editAsJson = !fitsForm(settings);
		tab = 'applies';
	});

	// The form tabs and the JSON tab edit the same settings; whichever was edited last is carried over when the
	// tab changes. A document the form cannot show keeps the JSON tab open and says why.
	function changeTab(next: string) {
		const target = next as RuleTab;
		if (target === tab) return;
		if (editAsJson && (target === 'triggers' || target === 'signals')) {
			switchToForm();
			if (editAsJson) return;
		}
		if (target === 'json' && !editAsJson) switchToJson();
		dialogError = '';
		tab = target;
	}

	let jsonTriggerCount = $derived.by(() => {
		try {
			const triggers = JSON.parse(settingsJson || '{}').triggers;
			return Array.isArray(triggers) ? triggers.length : 0;
		} catch {
			return 0;
		}
	});
	let triggerCount = $derived(editAsJson ? jsonTriggerCount : formTriggers.length);
	let conditionCount = $derived(rows.filter((row) => row.key.trim() !== '').length);
	let signalCount = $derived(
		Object.values(signalChoices).filter((choice) => choice !== 'inherit').length +
			[replayChoice, captureModeChoice, sessionsChoice, collectionChoice, metricsChoice].filter(
				(choice) => choice !== 'inherit'
			).length
	);

	function loadForm(settings: SettingsDocument) {
		formTriggers = JSON.parse(JSON.stringify(settings.triggers ?? []));
		baselinePercent =
			settings.sessions?.baselineSampleRate === undefined
				? undefined
				: settings.sessions.baselineSampleRate * 100;
		const next: Record<string, Choice> = {};
		for (const signal of SIGNALS)
			next[signal.key] = settings.signals?.[signal.key]?.mode ?? 'inherit';
		signalChoices = next;
		// A Flutter project is not offered the browser-only signals, but a rule that already sets one keeps
		// showing it, so the form can still change it.
		shownSignals = flutter
			? [
					...signalsFor(true),
					...SIGNALS.filter(
						(signal) => BROWSER_ONLY_SIGNALS.has(signal.key) && next[signal.key] !== 'inherit'
					)
				]
			: SIGNALS;
		replayChoice = settings.replay?.mode ?? 'inherit';
		captureModeChoice = settings.replay?.captureMode ?? 'inherit';
		// Only the Flutter SDK reads the replay type, but a rule that already sets one keeps showing it, so the
		// form can still change it.
		showCaptureMode = flutter || captureModeChoice !== 'inherit';
		sessionsChoice =
			settings.sessions?.enabled === undefined
				? 'inherit'
				: settings.sessions.enabled
					? 'on'
					: 'off';
		collectionChoice = settings.enabled === undefined ? 'inherit' : settings.enabled ? 'on' : 'off';
		metricsChoice =
			settings.metrics?.enabled === undefined ? 'inherit' : settings.metrics.enabled ? 'on' : 'off';
		metricsSamplePercent =
			settings.metrics?.sampleRate === undefined ? undefined : settings.metrics.sampleRate * 100;
	}

	function formSettings(): SettingsDocument {
		const settings: SettingsDocument = { schemaVersion: 1 };
		if (formTriggers.length) settings.triggers = formTriggers;
		const signals: NonNullable<SettingsDocument['signals']> = {};
		for (const [key, choice] of Object.entries(signalChoices)) {
			if (choice !== 'inherit') signals[key] = { mode: choice };
		}
		if (Object.keys(signals).length > 0) settings.signals = signals;
		const replay: NonNullable<SettingsDocument['replay']> = {};
		if (replayChoice !== 'inherit') replay.mode = replayChoice;
		if (captureModeChoice !== 'inherit') replay.captureMode = captureModeChoice;
		if (Object.keys(replay).length > 0) settings.replay = replay;
		if (sessionsChoice !== 'inherit') settings.sessions = { enabled: sessionsChoice === 'on' };
		const baseline = optionalNumber(baselinePercent);
		if (baseline !== undefined)
			settings.sessions = { ...settings.sessions, baselineSampleRate: baseline / 100 };
		if (collectionChoice !== 'inherit') settings.enabled = collectionChoice === 'on';
		if (metricsChoice !== 'inherit') settings.metrics = { enabled: metricsChoice === 'on' };
		const metricsSample = optionalNumber(metricsSamplePercent);
		if (metricsSample !== undefined)
			settings.metrics = { ...settings.metrics, sampleRate: metricsSample / 100 };
		return settings;
	}

	function switchToJson() {
		settingsJson = JSON.stringify(formSettings(), null, 2);
		editAsJson = true;
	}

	function switchToForm() {
		let parsed: SettingsDocument;
		try {
			parsed = JSON.parse(settingsJson);
		} catch {
			dialogError = 'The JSON does not parse; fix it before switching to the form.';
			return;
		}
		if (!fitsForm(parsed)) {
			dialogError =
				'This rule sets advanced sample rates or buffer sizes, which only the JSON editor can show.';
			return;
		}
		dialogError = '';
		loadForm(parsed);
		editAsJson = false;
	}

	function addTemplate(trigger: Record<string, unknown>) {
		let parsed: SettingsDocument;
		try {
			parsed = JSON.parse(settingsJson || '{}');
		} catch {
			dialogError = 'The JSON does not parse; fix it before adding a trigger.';
			return;
		}
		parsed.triggers = withTrigger(
			(parsed.triggers ?? []) as Array<{ id?: unknown }>,
			trigger,
			otherTriggerIds
		);
		settingsJson = JSON.stringify(parsed, null, 2);
	}

	function addLogTrigger(trigger: Record<string, unknown>) {
		if (!editAsJson) switchToJson();
		tab = 'json';
		addTemplate(trigger);
	}

	async function save() {
		const projectId = projectsState.currentProjectId;
		if (!projectId) return;
		let settings: SettingsDocument;
		if (editAsJson) {
			try {
				settings = JSON.parse(settingsJson);
			} catch {
				dialogError = 'Settings must be valid JSON.';
				return;
			}
		} else {
			settings = formSettings();
		}
		saving = true;
		dialogError = '';
		const body = {
			name,
			enabled,
			conditions: buildRuleConditions(rows, rolloutPercent, rolloutBy),
			settings
		};
		try {
			if (rule) {
				await api.put(`/sdk-config/rules/${rule.id}`, body, { projectId });
				toast.success('Successfully updated the Rule', { position: 'top-center' });
			} else {
				await api.post('/sdk-config/rules', body, { projectId });
				toast.success('Successfully created the Rule', { position: 'top-center' });
			}
			open = false;
			onSaved();
		} catch (e) {
			if (getErrorStatus(e) !== 403) {
				dialogError = getErrorMessage(e) || 'Failed to save the rule';
			}
		} finally {
			saving = false;
		}
	}
</script>

<datalist id="sdk-config-keys">
	{#each SUGGESTED_KEYS as key (key)}
		<option value={key}></option>
	{/each}
</datalist>

<Sheet.Root bind:open>
	<Sheet.Content
		side="right"
		class="flex w-full flex-col gap-0 overflow-y-auto sm:w-[760px] sm:max-w-[min(760px,100vw)]"
		interactOutsideBehavior={saving ? 'ignore' : 'close'}
	>
		<Sheet.Header class="border-b px-6 pb-4">
			<Sheet.Title>{rule ? 'Edit Rule' : 'New Rule'}</Sheet.Title>
			<Sheet.Description>
				Clients whose attributes match get these settings. Rules apply top to bottom, and a later
				rule overrides what an earlier one set. Settings only lower what the SDK's {flutter
					? 'TracewayOptions'
					: 'init() options'}
				allow.
			</Sheet.Description>
		</Sheet.Header>
		<div class="px-6 pt-5">
			<SdkVersionWarning />
		</div>
		<p
			class="px-6 pt-3 text-xs text-muted-foreground"
			title="SDK defaults are overlaid by matching rules in their displayed order. Later rules replace earlier fields. Temporary captures can increase collection, but cannot exceed developer-disabled signals, plan limits, or the per-session upload budget."
		>
			Settings combine with SDK defaults and earlier matching rules. Use Preview to inspect the
			server overlay.
		</p>

		<form
			class="flex-1 space-y-5 px-6 py-5"
			onsubmit={(e) => {
				e.preventDefault();
				save();
			}}
		>
			<ErrorAlert error={dialogError} />

			<div class="flex items-end gap-4">
				<div class="flex-1 space-y-2">
					<Label for="sdk-rule-name">Name</Label>
					<Input
						id="sdk-rule-name"
						bind:value={name}
						placeholder="Stream replay for the 2.4 beta"
						disabled={saving}
					/>
				</div>
				<div class="flex items-center gap-2 pb-2">
					<Switch id="sdk-rule-enabled" bind:checked={enabled} disabled={saving} />
					<Label for="sdk-rule-enabled">Enabled</Label>
				</div>
			</div>

			<Tabs.Root bind:value={() => tab, changeTab}>
				<Tabs.List class="w-full justify-start">
					<Tabs.Trigger value="applies">
						Applies to{conditionCount ? ` (${conditionCount})` : ''}
					</Tabs.Trigger>
					<Tabs.Trigger value="triggers">
						Triggers{triggerCount ? ` (${triggerCount})` : ''}
					</Tabs.Trigger>
					<Tabs.Trigger value="signals">
						Signals{!editAsJson && signalCount ? ` (${signalCount})` : ''}
					</Tabs.Trigger>
					<Tabs.Trigger value="json">JSON</Tabs.Trigger>
				</Tabs.List>
			</Tabs.Root>

			<div>
				{#if tab === 'applies'}
					<div class="space-y-3">
						<p class="text-xs text-muted-foreground">
							The clients this rule is for. Every condition has to match.
						</p>
						{#each rows as row, index (index)}
							<div class="flex items-center gap-2">
								<Input
									class="flex-1"
									list="sdk-config-keys"
									placeholder="user.id"
									bind:value={row.key}
									disabled={saving}
								/>
								<Select.Root type="single" bind:value={row.op} disabled={saving}>
									<Select.Trigger class="w-40">
										{CONDITION_OPS.find((o) => o.value === row.op)?.label ?? row.op}
									</Select.Trigger>
									<Select.Content>
										{#each CONDITION_OPS as option (option.value)}
											<Select.Item value={option.value}>{option.label}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
								{#if CONDITION_OPS.find((o) => o.value === row.op)?.needsValue !== 'none'}
									<Input
										class="flex-1"
										placeholder={row.op === 'version'
											? '>=2.4 <3'
											: row.op === 'in' || row.op === 'not_in'
												? 'a, b, c'
												: isNumericOp(row.op)
													? 'number'
													: 'value'}
										bind:value={row.value}
										disabled={saving}
									/>
								{/if}
								<Button
									type="button"
									variant="ghost"
									size="icon"
									aria-label="Remove condition"
									onclick={() => (rows = rows.filter((_, i) => i !== index))}
									disabled={saving}
								>
									<X class="h-4 w-4" />
								</Button>
							</div>
						{/each}
						<div class="flex flex-wrap items-center gap-3">
							<Button
								type="button"
								variant="outline"
								size="sm"
								onclick={() => (rows = [...rows, { key: '', op: 'eq', value: '' }])}
								disabled={saving}
							>
								<Plus class="mr-2 h-4 w-4" />
								Add condition
							</Button>
							<div class="flex items-center gap-2">
								<Label for="sdk-rule-rollout" class="text-sm font-normal text-muted-foreground"
									>Roll out to</Label
								>
								<Input
									id="sdk-rule-rollout"
									class="w-20"
									type="number"
									min="0"
									max="100"
									placeholder="100"
									bind:value={rolloutPercent}
									disabled={saving}
								/>
								<span class="text-sm text-muted-foreground">% of matching clients</span>
							</div>
						</div>
						{#if optionalNumber(rolloutPercent) !== undefined}
							<div class="space-y-2">
								<Label for="sdk-rule-rollout-by">Keep the same cohort by</Label>
								<Input
									id="sdk-rule-rollout-by"
									placeholder="Device (default), or user.id"
									bind:value={rolloutBy}
									disabled={saving}
								/>
							</div>
						{/if}
						{#if rows.length === 0}
							<p class="text-xs text-muted-foreground">
								No conditions: the rule applies to every client.
							</p>
						{/if}
					</div>
				{:else if tab === 'triggers'}
					<div class="space-y-4">
						{#if recordsSequences}
							<SequenceTriggerBuilder bind:triggers={formTriggers} disabled={saving} {flutter} />
						{/if}
						<LogTriggerBuilder disabled={saving} onAdd={addLogTrigger} {flutter} />
					</div>
				{:else if tab === 'signals'}
					<div class="space-y-4">
						<p class="text-xs text-muted-foreground">
							<strong>Buffer</strong> keeps telemetry on the device until a trigger or capture
							uploads it; <strong>Stream</strong> sends it straight away; <strong>Inherit</strong> leaves
							it to earlier rules and the SDK defaults.
						</p>
						<div class="grid gap-x-6 gap-y-2 sm:grid-cols-2">
							{#each shownSignals as signal (signal.key)}
								<div class="flex items-center justify-between gap-3">
									<div class="min-w-0">
										<div class="text-sm">{signal.label}</div>
										<div class="truncate text-xs text-muted-foreground">{signal.hint}</div>
									</div>
									<Select.Root
										type="single"
										bind:value={signalChoices[signal.key]}
										disabled={saving}
									>
										<Select.Trigger class="w-28"
											>{choiceLabels[signalChoices[signal.key]]}</Select.Trigger
										>
										<Select.Content>
											{#each Object.entries(choiceLabels) as [value, label] (value)}
												<Select.Item {value}>{label}</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								</div>
							{/each}
							<div class="flex items-center justify-between gap-3">
								<div>
									<div class="text-sm">Session replay</div>
									<div class="text-xs text-muted-foreground">
										{flutter ? 'Screen recording' : 'rrweb recording'}
									</div>
								</div>
								<Select.Root type="single" bind:value={replayChoice} disabled={saving}>
									<Select.Trigger class="w-28">{choiceLabels[replayChoice]}</Select.Trigger>
									<Select.Content>
										{#each Object.entries(choiceLabels) as [value, label] (value)}
											<Select.Item {value}>{label}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							</div>
							{#if showCaptureMode}
								<div class="flex items-center justify-between gap-3">
									<div>
										<div class="text-sm">Replay type</div>
										<div class="text-xs text-muted-foreground">
											{flutter ? 'Video, or a wireframe of the widget tree' : 'Flutter SDK only'}
										</div>
									</div>
									<Select.Root type="single" bind:value={captureModeChoice} disabled={saving}>
										<Select.Trigger class="w-28"
											>{captureModeLabels[captureModeChoice]}</Select.Trigger
										>
										<Select.Content>
											{#each Object.entries(captureModeLabels) as [value, label] (value)}
												<Select.Item {value}>{label}</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								</div>
							{/if}
							<div class="flex items-center justify-between gap-3">
								<div>
									<div class="text-sm">Sessions</div>
									<div class="text-xs text-muted-foreground">Session start and end</div>
								</div>
								<Select.Root type="single" bind:value={sessionsChoice} disabled={saving}>
									<Select.Trigger class="w-28">{toggleLabels[sessionsChoice]}</Select.Trigger>
									<Select.Content>
										{#each Object.entries(toggleLabels) as [value, label] (value)}
											<Select.Item {value}>{label}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							</div>
							<div class="flex items-center justify-between gap-3">
								<div>
									<div class="text-sm">Performance metrics</div>
									<div class="text-xs text-muted-foreground">
										Aggregated on the device, one small upload per flush
									</div>
								</div>
								<Select.Root type="single" bind:value={metricsChoice} disabled={saving}>
									<Select.Trigger class="w-28">{toggleLabels[metricsChoice]}</Select.Trigger>
									<Select.Content>
										{#each Object.entries(toggleLabels) as [value, label] (value)}
											<Select.Item {value}>{label}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							</div>
							<div class="flex items-center justify-between gap-3">
								<div>
									<div class="text-sm">All collection</div>
									<div class="text-xs text-muted-foreground">Off sends nothing at all</div>
								</div>
								<Select.Root type="single" bind:value={collectionChoice} disabled={saving}>
									<Select.Trigger class="w-28">{toggleLabels[collectionChoice]}</Select.Trigger>
									<Select.Content>
										{#each Object.entries(toggleLabels) as [value, label] (value)}
											<Select.Item {value}>{label}</Select.Item>
										{/each}
									</Select.Content>
								</Select.Root>
							</div>
						</div>
						<div class="space-y-2">
							<Label for="sdk-rule-baseline">Healthy session sample (%)</Label>
							<Button
								type="button"
								variant="outline"
								size="sm"
								disabled={saving}
								onclick={() => (baselinePercent = 1)}>Use 1% baseline</Button
							>
							<Input
								id="sdk-rule-baseline"
								type="number"
								min="0"
								max="100"
								step="0.1"
								bind:value={baselinePercent}
								placeholder="Inherit (0% by default)"
							/>
							<p class="text-xs text-muted-foreground">
								Try 1% for a stable sample of session breadcrumbs and request spans. Errors and {flutter
									? 'performance metrics'
									: 'web vitals'} stay on; replay stays error-triggered unless configured separately.
							</p>
						</div>
						<div class="space-y-2">
							<Label for="sdk-rule-metrics-sample">Sessions that record metrics (%)</Label>
							<Input
								id="sdk-rule-metrics-sample"
								type="number"
								min="0"
								max="100"
								step="0.1"
								bind:value={metricsSamplePercent}
								placeholder="Inherit (100% by default)"
							/>
							<p class="text-xs text-muted-foreground">
								Counts are scaled back up by the sample rate, so charts stay comparable. Lower it
								for very high traffic. Use 0 for none, or at least 0.1.
							</p>
						</div>
					</div>
				{:else}
					<div class="space-y-3">
						<p class="text-xs text-muted-foreground">
							The whole settings document. Sample rates, buffer sizes and triggers the form cannot
							build are edited here.
						</p>
						<div class="flex flex-wrap items-center gap-2">
							<span class="text-xs text-muted-foreground">Add a trigger:</span>
							{#each triggerTemplates as template (template.label)}
								<Button
									type="button"
									variant="outline"
									size="sm"
									onclick={() => addTemplate(template.trigger)}
									disabled={saving}
								>
									<Plus class="mr-1 h-3.5 w-3.5" />
									{template.label}
								</Button>
							{/each}
						</div>
						<CodeEditor bind:value={settingsJson} minHeight="260px" />
					</div>
				{/if}
			</div>
		</form>

		<div class="sticky bottom-0 flex justify-end gap-2 border-t bg-background px-6 py-4">
			<Button variant="outline" onclick={() => (open = false)} disabled={saving}>Cancel</Button>
			{#if rule}
				<Button onclick={save} disabled={saving}>
					<Check class="mr-2 h-4 w-4" />
					{saving ? 'Saving...' : 'Update Rule'}
				</Button>
			{:else}
				<Button variant="success" onclick={save} disabled={saving}>
					<Plus class="mr-2 h-4 w-4" />
					{saving ? 'Creating...' : 'New Rule'}
				</Button>
			{/if}
		</div>
	</Sheet.Content>
</Sheet.Root>
