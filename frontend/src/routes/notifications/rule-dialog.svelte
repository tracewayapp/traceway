<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import { Plus, Check, TriangleAlert } from '@lucide/svelte';
	import * as Alert from '$lib/components/ui/alert';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState, isFrontendFramework } from '$lib/state/projects.svelte';
	import { ruleTypeOptions } from './rule-types';

	interface NotificationChannel {
		id: number;
		projectId: string;
		name: string;
		channelType: string;
		config: any;
		enabled: boolean;
		createdAt: string;
	}

	interface NotificationRule {
		id: number;
		projectId: string;
		channelId: number;
		name: string;
		ruleType: string;
		config: any;
		enabled: boolean;
		cooldownMinutes: number;
		severity: string;
		snoozedUntil: string | null;
		channelName: string;
		channelType: string;
		createdAt: string;
	}

	interface Props {
		open: boolean;
		rule: NotificationRule | null;
		channels: NotificationChannel[];
		onSaved: () => void;
	}

	let { open = $bindable(), rule, channels, onSaved }: Props = $props();

	let name = $state('');
	let channelId = $state('');
	let ruleType = $state('new_error');
	let cooldownMinutes = $state(15);
	let severity = $state('');
	let loading = $state(false);
	let error = $state('');

	const severityOptions = [
		{ value: '', label: 'Auto (default)' },
		{ value: 'critical', label: 'Critical' },
		{ value: 'warning', label: 'Warning' },
		{ value: 'info', label: 'Info' }
	];

	let thresholdPercent = $state(5);
	let lookbackMinutes = $state(5);
	let minRequests = $state(10);
	let endpoint = $state('*');
	let thresholdMs = $state(500);
	let thresholdApdex = $state(0.85);
	let metricName = $state('');
	let operator = $state('gt');
	let thresholdValue = $state(90);
	let aggregation = $state('avg');
	let dataType = $state('any');
	let silenceMinutes = $state(10);
	let thresholdCount = $state(100);
	let taskName = $state('*');
	let minExecutions = $state(5);
	let dropPercent = $state(50);
	let baselineWindowMinutes = $state(60);
	let ignorePatterns = $state('');
	let traceName = $state('*');
	let thresholdCost = $state(0.5);

	const isEditing = $derived(rule !== null);

	const isFrontendProject = $derived(
		!!projectsState.currentProject &&
			isFrontendFramework(projectsState.currentProject.framework)
	);

	const visibleRuleTypes = $derived(
		isFrontendProject ? [{ value: 'new_error', label: 'New Issue' }] : ruleTypeOptions
	);

	$effect(() => {
		if (open && !visibleRuleTypes.some((o) => o.value === ruleType)) {
			ruleType = 'new_error';
		}
	});

	const ruleTypeDescriptions: Record<string, string> = {
		new_error:
			'Fires when a completely new error type is detected for the first time in your project.',
		impact_score_critical:
			'Fires when an endpoint\u2019s impact score reaches critical level (\u2265 0.75), indicating severe performance or reliability degradation.',
		impact_score_high:
			'Fires when an endpoint\u2019s impact score reaches high level (\u2265 0.50), indicating significant performance or reliability issues.',
		impact_score_medium:
			'Fires when an endpoint\u2019s impact score reaches medium level (\u2265 0.25), an early warning of emerging performance or reliability issues.',
		ai_trace_cost:
			'Fires when an individual AI trace exceeds a cost threshold. Monitors per-call spending across AI providers.',
		error_regression:
			'Fires when an error that was previously archived (resolved) occurs again.',
		error_rate_threshold:
			'Fires when the percentage of requests returning 5xx across all endpoints exceeds the threshold within the lookback window.',
		error_count_threshold:
			'Fires when the total number of errors recorded within the lookback window reaches the threshold.',
		endpoint_p95_threshold:
			'Fires when the P95 response time of an endpoint (or all endpoints) exceeds the threshold within the lookback window.',
		endpoint_p99_threshold:
			'Fires when the P99 response time of an endpoint (or all endpoints) exceeds the threshold within the lookback window.',
		endpoint_error_rate:
			'Fires when the percentage of 5xx responses on a specific endpoint exceeds the threshold within the lookback window.',
		apdex_drop:
			'Fires when the Apdex satisfaction score drops below the threshold within the lookback window.',
		metric_threshold:
			'Fires when an aggregated metric value violates the configured comparison against the threshold within the lookback window.',
		no_data:
			'Fires when no telemetry of the selected kind (endpoints, exceptions, metrics or tasks) has been received for the configured number of minutes.',
		task_duration_threshold:
			'Fires when the P95 duration of a background task exceeds the threshold within the lookback window.',
		task_failure_rate:
			'Fires when the percentage of background task executions that recorded an error exceeds the threshold within the lookback window.',
		throughput_drop:
			'Fires when request throughput drops by more than the configured percentage compared to the preceding baseline window.'
	};

	const operatorOptions = [
		{ value: 'gt', label: '> Greater than' },
		{ value: 'gte', label: '>= Greater or equal' },
		{ value: 'lt', label: '< Less than' },
		{ value: 'lte', label: '<= Less or equal' },
		{ value: 'eq', label: '= Equal' }
	];

	const aggregationOptions = [
		{ value: 'avg', label: 'Average' },
		{ value: 'max', label: 'Maximum' },
		{ value: 'min', label: 'Minimum' },
		{ value: 'sum', label: 'Sum' },
		{ value: 'p95', label: 'P95' },
		{ value: 'p99', label: 'P99' },
		{ value: 'last', label: 'Last' }
	];

	const dataTypeOptions = [
		{ value: 'any', label: 'Any' },
		{ value: 'endpoints', label: 'Endpoints' },
		{ value: 'exceptions', label: 'Exceptions' },
		{ value: 'metrics', label: 'Metrics' },
		{ value: 'tasks', label: 'Tasks' }
	];

	function resetForm() {
		name = '';
		channelId = '';
		ruleType = 'new_error';
		cooldownMinutes = 15;
		severity = '';
		error = '';
		thresholdPercent = 5;
		lookbackMinutes = 5;
		minRequests = 10;
		endpoint = '*';
		thresholdMs = 500;
		thresholdApdex = 0.85;
		metricName = '';
		operator = 'gt';
		thresholdValue = 90;
		aggregation = 'avg';
		dataType = 'any';
		silenceMinutes = 10;
		thresholdCount = 100;
		taskName = '*';
		minExecutions = 5;
		dropPercent = 50;
		baselineWindowMinutes = 60;
		ignorePatterns = '';
		traceName = '*';
		thresholdCost = 0.5;
	}

	function populateFromRule(r: NotificationRule) {
		name = r.name;
		channelId = r.channelId.toString();
		ruleType = r.ruleType;
		cooldownMinutes = r.cooldownMinutes;
		severity = r.severity || '';
		const cfg = r.config || {};

		switch (r.ruleType) {
			case 'error_rate_threshold':
				thresholdPercent = cfg.thresholdPercent ?? 5;
				lookbackMinutes = cfg.lookbackMinutes ?? 5;
				minRequests = cfg.minRequests ?? 10;
				break;
			case 'endpoint_p95_threshold':
			case 'endpoint_p99_threshold':
				endpoint = cfg.endpoint ?? '*';
				thresholdMs = cfg.thresholdMs ?? 500;
				lookbackMinutes = cfg.lookbackMinutes ?? 5;
				break;
			case 'apdex_drop':
				thresholdApdex = cfg.thresholdApdex ?? 0.85;
				lookbackMinutes = cfg.lookbackMinutes ?? 15;
				minRequests = cfg.minRequests ?? 50;
				break;
			case 'metric_threshold':
				metricName = cfg.metricName ?? '';
				operator = cfg.operator ?? 'gt';
				thresholdValue = cfg.thresholdValue ?? 90;
				aggregation = cfg.aggregation ?? 'avg';
				lookbackMinutes = cfg.lookbackMinutes ?? 5;
				break;
			case 'no_data':
				dataType = cfg.dataType ?? 'any';
				silenceMinutes = cfg.silenceMinutes ?? 10;
				break;
			case 'error_count_threshold':
				thresholdCount = cfg.thresholdCount ?? 100;
				lookbackMinutes = cfg.lookbackMinutes ?? 60;
				break;
			case 'task_duration_threshold':
				taskName = cfg.taskName ?? '*';
				thresholdMs = cfg.thresholdMs ?? 30000;
				lookbackMinutes = cfg.lookbackMinutes ?? 30;
				break;
			case 'task_failure_rate':
				taskName = cfg.taskName ?? '*';
				thresholdPercent = cfg.thresholdPercent ?? 10;
				lookbackMinutes = cfg.lookbackMinutes ?? 60;
				minExecutions = cfg.minExecutions ?? 5;
				break;
			case 'throughput_drop':
				dropPercent = cfg.dropPercent ?? 50;
				lookbackMinutes = cfg.lookbackMinutes ?? 15;
				baselineWindowMinutes = cfg.baselineWindowMinutes ?? 60;
				break;
			case 'endpoint_error_rate':
				endpoint = cfg.endpoint ?? '*';
				thresholdPercent = cfg.thresholdPercent ?? 2;
				lookbackMinutes = cfg.lookbackMinutes ?? 10;
				minRequests = cfg.minRequests ?? 20;
				break;
			case 'new_error':
				ignorePatterns = (cfg.ignorePatterns || []).join(', ');
				break;
			case 'impact_score_critical':
			case 'impact_score_high':
			case 'impact_score_medium':
				minRequests = cfg.minRequests ?? 50;
				break;
			case 'ai_trace_cost':
				traceName = cfg.traceName ?? '*';
				thresholdCost = cfg.thresholdCost ?? 0.5;
				break;
		}
	}

	function buildConfig(): any {
		switch (ruleType) {
			case 'error_rate_threshold':
				return { thresholdPercent, lookbackMinutes, minRequests };
			case 'endpoint_p95_threshold':
			case 'endpoint_p99_threshold':
				return { endpoint, thresholdMs, lookbackMinutes };
			case 'apdex_drop':
				return { thresholdApdex, lookbackMinutes, minRequests };
			case 'metric_threshold':
				return { metricName, operator, thresholdValue, aggregation, lookbackMinutes, tags: {} };
			case 'no_data':
				return { dataType, silenceMinutes };
			case 'error_count_threshold':
				return { thresholdCount, lookbackMinutes };
			case 'task_duration_threshold':
				return { taskName, thresholdMs, lookbackMinutes };
			case 'task_failure_rate':
				return { taskName, thresholdPercent, lookbackMinutes, minExecutions };
			case 'throughput_drop':
				return { dropPercent, lookbackMinutes, baselineWindowMinutes };
			case 'endpoint_error_rate':
				return { endpoint, thresholdPercent, lookbackMinutes, minRequests };
			case 'new_error': {
				const patterns = ignorePatterns
					.split(',')
					.map((p) => p.trim())
					.filter((p) => p);
				return { ignorePatterns: patterns };
			}
			case 'impact_score_critical':
			case 'impact_score_high':
			case 'impact_score_medium':
				return { minRequests };
			case 'ai_trace_cost':
				return { traceName, thresholdCost };
			default:
				return {};
		}
	}

	async function handleSubmit() {
		loading = true;
		error = '';

		try {
			const body = {
				channelId: parseInt(channelId),
				name,
				ruleType,
				config: buildConfig(),
				cooldownMinutes,
				severity
			};

			if (isEditing) {
				await api.put(`/notification-rules/${rule!.id}`, body, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully updated the Rule', { position: 'top-center' });
			} else {
				await api.post('/notification-rules', body, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully created the Rule', { position: 'top-center' });
			}
			onSaved();
		} catch (e: unknown) {
			error = e instanceof Error ? e.message : 'Failed to save rule';
		} finally {
			loading = false;
		}
	}

	function handleOpenChange(isOpen: boolean) {
		if (!isOpen) {
			resetForm();
		} else if (rule) {
			populateFromRule(rule);
		} else {
			resetForm();
		}
		open = isOpen;
	}

	$effect(() => {
		if (open && rule) {
			populateFromRule(rule);
		} else if (open && !rule) {
			resetForm();
		}
	});
</script>

<AlertDialog.Root {open} onOpenChange={handleOpenChange}>
	<AlertDialog.Content class="max-w-lg max-h-[85vh] overflow-y-auto">
		<AlertDialog.Header>
			<AlertDialog.Title>{isEditing ? 'Edit Rule' : 'New Rule'}</AlertDialog.Title>
			<AlertDialog.Description>
				{isEditing
					? 'Update the notification rule configuration'
					: 'Configure a new notification rule'}
			</AlertDialog.Description>
		</AlertDialog.Header>

		<form
			onsubmit={(e) => {
				e.preventDefault();
				handleSubmit();
			}}
			class="space-y-4"
		>
			<div class="space-y-2">
				<Label for="rule-name">Name</Label>
				<Input
					id="rule-name"
					bind:value={name}
					placeholder="e.g. High Error Rate"
					required
				/>
			</div>

			<div class="space-y-2">
				<Label for="rule-channel">Channel</Label>
				<Select.Root type="single" bind:value={channelId}>
					<Select.Trigger class="w-full">
						{channels.find((c) => c.id.toString() === channelId)?.name ||
							'Select channel'}
					</Select.Trigger>
					<Select.Content>
						{#each channels as ch}
							<Select.Item value={ch.id.toString()}>{ch.name}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			<div class="space-y-2">
				<Label for="rule-type">Rule Type</Label>
				<Select.Root type="single" bind:value={ruleType}>
					<Select.Trigger class="w-full">
						{ruleTypeOptions.find((o) => o.value === ruleType)?.label || 'Select type'}
					</Select.Trigger>
					<Select.Content>
						{#each visibleRuleTypes as option}
							<Select.Item value={option.value}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			{#if ruleTypeDescriptions[ruleType]}
				<Alert.Root class="bg-amber-50 border-amber-200 text-amber-900 dark:bg-amber-950/50 dark:border-amber-800 dark:text-amber-200 ">
					<TriangleAlert class="text-amber-600 dark:text-amber-400" />
					<Alert.Description class="text-amber-800 dark:text-amber-300">{ruleTypeDescriptions[ruleType]}</Alert.Description>
				</Alert.Root>
			{/if}

			<div class="rounded-md border p-3 space-y-3">
				<p class="text-sm font-medium text-muted-foreground">Rule Configuration</p>

				{#if ruleType === 'error_rate_threshold'}
					<div class="space-y-2">
						<Label for="cfg-threshold">Threshold (%)</Label>
						<Input
							id="cfg-threshold"
							type="number"
							bind:value={thresholdPercent}
							step="0.1"
							min="0"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-min-req">Min Requests</Label>
						<Input
							id="cfg-min-req"
							type="number"
							bind:value={minRequests}
							min="1"
						/>
					</div>
				{:else if ruleType === 'endpoint_p95_threshold' || ruleType === 'endpoint_p99_threshold'}
					<div class="space-y-2">
						<Label for="cfg-endpoint">Endpoint (* for all)</Label>
						<Input id="cfg-endpoint" bind:value={endpoint} placeholder="GET /api/users" />
					</div>
					<div class="space-y-2">
						<Label for="cfg-threshold-ms">Threshold (ms)</Label>
						<Input
							id="cfg-threshold-ms"
							type="number"
							bind:value={thresholdMs}
							min="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback-p">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback-p"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
				{:else if ruleType === 'apdex_drop'}
					<div class="space-y-2">
						<Label for="cfg-apdex">Apdex Threshold</Label>
						<Input
							id="cfg-apdex"
							type="number"
							bind:value={thresholdApdex}
							step="0.01"
							min="0"
							max="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback-a">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback-a"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-min-req-a">Min Requests</Label>
						<Input
							id="cfg-min-req-a"
							type="number"
							bind:value={minRequests}
							min="1"
						/>
					</div>
				{:else if ruleType === 'metric_threshold'}
					<div class="space-y-2">
						<Label for="cfg-metric">Metric Name</Label>
						<Input
							id="cfg-metric"
							bind:value={metricName}
							placeholder="cpu.used_pcnt"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-operator">Operator</Label>
						<Select.Root type="single" bind:value={operator}>
							<Select.Trigger class="w-full">
								{operatorOptions.find((o) => o.value === operator)?.label || operator}
							</Select.Trigger>
							<Select.Content>
								{#each operatorOptions as option}
									<Select.Item value={option.value}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					<div class="space-y-2">
						<Label for="cfg-value">Threshold Value</Label>
						<Input
							id="cfg-value"
							type="number"
							bind:value={thresholdValue}
							step="0.1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-agg">Aggregation</Label>
						<Select.Root type="single" bind:value={aggregation}>
							<Select.Trigger class="w-full">
								{aggregationOptions.find((o) => o.value === aggregation)?.label ||
									aggregation}
							</Select.Trigger>
							<Select.Content>
								{#each aggregationOptions as option}
									<Select.Item value={option.value}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback-m">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback-m"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
				{:else if ruleType === 'no_data'}
					<div class="space-y-2">
						<Label for="cfg-data-type">Data Type</Label>
						<Select.Root type="single" bind:value={dataType}>
							<Select.Trigger class="w-full">
								{dataTypeOptions.find((o) => o.value === dataType)?.label || dataType}
							</Select.Trigger>
							<Select.Content>
								{#each dataTypeOptions as option}
									<Select.Item value={option.value}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					<div class="space-y-2">
						<Label for="cfg-silence">Silence (minutes)</Label>
						<Input
							id="cfg-silence"
							type="number"
							bind:value={silenceMinutes}
							min="1"
						/>
					</div>
				{:else if ruleType === 'error_count_threshold'}
					<div class="space-y-2">
						<Label for="cfg-count">Threshold Count</Label>
						<Input
							id="cfg-count"
							type="number"
							bind:value={thresholdCount}
							min="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback-ec">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback-ec"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
				{:else if ruleType === 'task_duration_threshold'}
					<div class="space-y-2">
						<Label for="cfg-task">Task Name (* for all)</Label>
						<Input id="cfg-task" bind:value={taskName} placeholder="sync_users" />
					</div>
					<div class="space-y-2">
						<Label for="cfg-threshold-ms-t">Threshold (ms)</Label>
						<Input
							id="cfg-threshold-ms-t"
							type="number"
							bind:value={thresholdMs}
							min="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback-td">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback-td"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
				{:else if ruleType === 'task_failure_rate'}
					<div class="space-y-2">
						<Label for="cfg-task-f">Task Name (* for all)</Label>
						<Input id="cfg-task-f" bind:value={taskName} placeholder="sync_users" />
					</div>
					<div class="space-y-2">
						<Label for="cfg-threshold-pct">Threshold (%)</Label>
						<Input
							id="cfg-threshold-pct"
							type="number"
							bind:value={thresholdPercent}
							step="0.1"
							min="0"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback-tf">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback-tf"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-min-exec">Min Executions</Label>
						<Input
							id="cfg-min-exec"
							type="number"
							bind:value={minExecutions}
							min="1"
						/>
					</div>
				{:else if ruleType === 'throughput_drop'}
					<div class="space-y-2">
						<Label for="cfg-drop">Drop (%)</Label>
						<Input
							id="cfg-drop"
							type="number"
							bind:value={dropPercent}
							step="0.1"
							min="0"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback-tp">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback-tp"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-baseline">Baseline Window (minutes)</Label>
						<Input
							id="cfg-baseline"
							type="number"
							bind:value={baselineWindowMinutes}
							min="1"
						/>
					</div>
				{:else if ruleType === 'endpoint_error_rate'}
					<div class="space-y-2">
						<Label for="cfg-endpoint-er">Endpoint</Label>
						<Input
							id="cfg-endpoint-er"
							bind:value={endpoint}
							placeholder="POST /api/checkout"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-threshold-er">Threshold (%)</Label>
						<Input
							id="cfg-threshold-er"
							type="number"
							bind:value={thresholdPercent}
							step="0.1"
							min="0"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-lookback-er">Lookback (minutes)</Label>
						<Input
							id="cfg-lookback-er"
							type="number"
							bind:value={lookbackMinutes}
							min="1"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-min-req-er">Min Requests</Label>
						<Input
							id="cfg-min-req-er"
							type="number"
							bind:value={minRequests}
							min="1"
						/>
					</div>
				{:else if ruleType === 'new_error'}
					<div class="space-y-2">
						<Label for="cfg-ignore">Ignore Patterns (optional, comma-separated)</Label>
						<Input
							id="cfg-ignore"
							bind:value={ignorePatterns}
							placeholder="*timeout*, *context canceled*"
						/>
					</div>
				{:else if ruleType === 'impact_score_critical' || ruleType === 'impact_score_high' || ruleType === 'impact_score_medium'}
					<div class="space-y-2">
						<Label for="cfg-min-req-impact">Min Requests</Label>
						<Input
							id="cfg-min-req-impact"
							type="number"
							bind:value={minRequests}
							min="1"
						/>
					</div>
				{:else if ruleType === 'ai_trace_cost'}
					<div class="space-y-2">
						<Label for="cfg-trace-name">Trace Name (* for all)</Label>
						<Input
							id="cfg-trace-name"
							bind:value={traceName}
							placeholder="Customer Support Agent"
						/>
					</div>
					<div class="space-y-2">
						<Label for="cfg-threshold-cost">Cost Threshold ($)</Label>
						<Input
							id="cfg-threshold-cost"
							type="number"
							bind:value={thresholdCost}
							step="0.01"
							min="0"
						/>
					</div>
				{/if}
			</div>

			<div class="space-y-2">
				<Label for="rule-cooldown">Cooldown (minutes)</Label>
				<Input
					id="rule-cooldown"
					type="number"
					bind:value={cooldownMinutes}
					min="1"
				/>
			</div>

			<div class="space-y-2">
				<Label for="rule-severity">Severity</Label>
				<Select.Root type="single" bind:value={severity}>
					<Select.Trigger class="w-full">
						{severityOptions.find((o) => o.value === severity)?.label || 'Auto (default)'}
					</Select.Trigger>
					<Select.Content>
						{#each severityOptions as option}
							<Select.Item value={option.value}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>

			{#if error}
				<p class="text-sm text-destructive">{error}</p>
			{/if}
		</form>

		<AlertDialog.Footer>
			<AlertDialog.Cancel disabled={loading}>Cancel</AlertDialog.Cancel>
			<Button onclick={handleSubmit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{#if loading}
						Updating...
					{:else}
						Update Rule
					{/if}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{#if loading}
						Creating...
					{:else}
						New Rule
					{/if}
				{/if}
			</Button>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
