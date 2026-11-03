<script lang="ts">
	import SdkVersionWarning from './sdk-version-warning.svelte';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import * as Select from '$lib/components/ui/select';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { CodeEditor } from '$lib/components/ui/code-editor';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { Check, Plus, X } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { getErrorMessage, getErrorStatus } from '$lib/utils/errors';
	import { projectsState } from '$lib/state/projects.svelte';
	import LogTriggerBuilder from './log-trigger-builder.svelte';
	import {
		CONDITION_OPS,
		SIGNALS,
		SUGGESTED_KEYS,
		TRIGGER_TEMPLATES,
		fitsForm,
		uniqueTriggerId,
		buildRuleConditions,
		conditionText,
		type ConditionRow,
		type Mode,
		type SdkConfigRule,
		type SettingsDocument
	} from './sdk-config';

	interface Props {
		open: boolean;
		rule: SdkConfigRule | null;
		onSaved: () => void;
	}

	let { open = $bindable(), rule, onSaved }: Props = $props();

	type Choice = 'inherit' | Mode;

	let name = $state('');
	let enabled = $state(true);
	let rows = $state<ConditionRow[]>([]);
	let rolloutPercent = $state<number | undefined>(undefined);
	let rolloutBy = $state('');
	let baselinePercent = $state<number | undefined>(undefined);
	let editAsJson = $state(false);
	let signalChoices = $state<Record<string, Choice>>({});
	let replayChoice = $state<Choice>('inherit');
	let sessionsChoice = $state<'inherit' | 'on' | 'off'>('inherit');
	let collectionChoice = $state<'inherit' | 'on' | 'off'>('inherit');
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
	});

	function loadForm(settings: SettingsDocument) {
		baselinePercent =
			settings.sessions?.baselineSampleRate === undefined
				? undefined
				: settings.sessions.baselineSampleRate * 100;
		const next: Record<string, Choice> = {};
		for (const signal of SIGNALS)
			next[signal.key] = settings.signals?.[signal.key]?.mode ?? 'inherit';
		signalChoices = next;
		replayChoice = settings.replay?.mode ?? 'inherit';
		sessionsChoice =
			settings.sessions?.enabled === undefined
				? 'inherit'
				: settings.sessions.enabled
					? 'on'
					: 'off';
		collectionChoice = settings.enabled === undefined ? 'inherit' : settings.enabled ? 'on' : 'off';
	}

	function formSettings(): SettingsDocument {
		const settings: SettingsDocument = { schemaVersion: 1 };
		const signals: NonNullable<SettingsDocument['signals']> = {};
		for (const [key, choice] of Object.entries(signalChoices)) {
			if (choice !== 'inherit') signals[key] = { mode: choice };
		}
		if (Object.keys(signals).length > 0) settings.signals = signals;
		if (replayChoice !== 'inherit') settings.replay = { mode: replayChoice };
		if (sessionsChoice !== 'inherit') settings.sessions = { enabled: sessionsChoice === 'on' };
		if (baselinePercent !== undefined)
			settings.sessions = { ...settings.sessions, baselineSampleRate: baselinePercent / 100 };
		if (collectionChoice !== 'inherit') settings.enabled = collectionChoice === 'on';
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
				'This rule sets triggers, sample rates or buffer sizes, which only the JSON editor can show.';
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
		const triggers = (parsed.triggers ?? []) as Array<{ id?: unknown }>;
		const added = {
			...structuredClone(trigger),
			id: uniqueTriggerId(String(trigger.id), triggers)
		};
		parsed.triggers = [...triggers, added];
		settingsJson = JSON.stringify(parsed, null, 2);
	}

	function addLogTrigger(trigger: Record<string, unknown>) {
		if (!editAsJson) switchToJson();
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

<AlertDialog.Root bind:open>
	<AlertDialog.Content
		class="max-h-[90vh] overflow-y-auto sm:max-w-2xl"
		interactOutsideBehavior={saving ? 'ignore' : 'close'}
	>
		<AlertDialog.Header>
			<AlertDialog.Title>{rule ? 'Edit Rule' : 'New Rule'}</AlertDialog.Title>
			<AlertDialog.Description>
				Clients whose attributes match get these settings. Rules apply top to bottom, and a later
				rule overrides what an earlier one set. Settings only lower what the SDK's init() options
				allow.
			</AlertDialog.Description>
		</AlertDialog.Header>
		<SdkVersionWarning />

		<form
			class="space-y-5"
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

			<div class="space-y-2">
				<Label>Applies to</Label>
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
				{#if rolloutPercent !== undefined}
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

			<div class="space-y-3">
				<div class="flex items-center justify-between">
					<Label>Settings</Label>
					<Button
						type="button"
						variant="link"
						size="sm"
						class="h-auto p-0"
						onclick={() => (editAsJson ? switchToForm() : switchToJson())}
						disabled={saving}
					>
						{editAsJson ? 'Edit as form' : 'Edit as JSON (triggers, sample rates)'}
					</Button>
				</div>

				{#if editAsJson}
					<div class="flex flex-wrap items-center gap-2">
						<span class="text-xs text-muted-foreground">Add a trigger:</span>
						{#each TRIGGER_TEMPLATES as template (template.label)}
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
					<LogTriggerBuilder disabled={saving} onAdd={addLogTrigger} />
					<CodeEditor bind:value={settingsJson} minHeight="260px" />
				{:else}
					<p class="text-xs text-muted-foreground">
						<strong>Buffer</strong> keeps telemetry on the device until a trigger or capture uploads
						it; <strong>Stream</strong> sends it straight away; <strong>Inherit</strong> leaves it to
						earlier rules and the SDK defaults.
					</p>
					<div class="grid gap-x-6 gap-y-2 sm:grid-cols-2">
						{#each SIGNALS as signal (signal.key)}
							<div class="flex items-center justify-between gap-3">
								<div class="min-w-0">
									<div class="text-sm">{signal.label}</div>
									<div class="truncate text-xs text-muted-foreground">{signal.hint}</div>
								</div>
								<Select.Root type="single" bind:value={signalChoices[signal.key]} disabled={saving}>
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
								<div class="text-xs text-muted-foreground">rrweb recording</div>
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
							Try 1% for a stable sample of session breadcrumbs and request spans. Errors and web
							vitals stay on; replay stays error-triggered unless configured separately.
						</p>
					</div>
					<LogTriggerBuilder disabled={saving} onAdd={addLogTrigger} />
				{/if}
			</div>
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={saving}>Cancel</AlertDialog.Cancel>
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
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
