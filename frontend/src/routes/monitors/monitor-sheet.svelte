<script lang="ts">
	import { api } from '$lib/api';
	import * as Sheet from '$lib/components/ui/sheet';
	import * as Select from '$lib/components/ui/select';
	import * as Tabs from '$lib/components/ui/tabs';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { ErrorAlert } from '$lib/components/ui/error-alert';
	import { CodeEditor } from '$lib/components/ui/code-editor';
	import KeyValueRows from '$lib/components/synthetics/key-value-rows.svelte';
	import { underlineTabTriggerClass, underlineTabListClass } from '$lib/utils/tabs';
	import { projectsState } from '$lib/state/projects.svelte';
	import type { SyntheticCheck, CheckConfig, CheckType } from '$lib/state/monitors.svelte';
	import { Check, Globe, Plus } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	interface Props {
		open: boolean;
		check: SyntheticCheck | null;
		onSaved: () => void;
	}

	let { open = $bindable(), check, onSaved }: Props = $props();

	const isEditing = $derived(check !== null);
	let loading = $state(false);
	let error = $state('');

	// Common fields
	let name = $state('');
	let checkType = $state<CheckType>('http');
	let intervalSeconds = $state('60');
	let timeoutSeconds = $state('30');
	let failureThreshold = $state('2');

	// HTTP fields
	let requestTab = $state('headers');
	let httpUrl = $state('');
	let httpMethod = $state('GET');
	let headerRows = $state<{ key: string; value: string }[]>([]);
	let authType = $state('none');
	let authUsername = $state('');
	let authPassword = $state('');
	let authToken = $state('');
	let authHeaderName = $state('');
	let authHeaderValue = $state('');
	let httpBody = $state('');
	let httpBodyContentType = $state('application/json');
	let followRedirects = $state(true);
	let skipTlsVerify = $state(false);
	let expectedStatusCodes = $state('');
	let bodyContains = $state('');
	let bodyRegex = $state('');
	let maxResponseTimeMs = $state('');
	let tlsMinDaysRemaining = $state('');

	// TCP fields
	let tcpHost = $state('');
	let tcpPort = $state('');
	let tcpUseTls = $state(false);
	let tcpSendPayload = $state('');
	let tcpExpectContains = $state('');
	let tcpTlsMinDaysRemaining = $state('');

	// Browser fields
	let browserScript = $state('');
	let envRows = $state<{ key: string; value: string }[]>([]);

	const checkTypeOptions = [
		{ value: 'http', label: 'HTTP' },
		{ value: 'tcp', label: 'TCP' },
		{ value: 'browser', label: 'Browser (Playwright)' }
	];

	const intervalOptions = [
		{ value: '30', label: 'Every 30 seconds' },
		{ value: '60', label: 'Every minute' },
		{ value: '120', label: 'Every 2 minutes' },
		{ value: '300', label: 'Every 5 minutes' },
		{ value: '600', label: 'Every 10 minutes' },
		{ value: '900', label: 'Every 15 minutes' },
		{ value: '1800', label: 'Every 30 minutes' },
		{ value: '3600', label: 'Every hour' },
		{ value: '21600', label: 'Every 6 hours' },
		{ value: '86400', label: 'Every 24 hours' }
	];

	const methodOptions = ['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'];

	const authOptions = [
		{ value: 'none', label: 'No auth' },
		{ value: 'basic', label: 'Basic auth' },
		{ value: 'bearer', label: 'Bearer token' },
		{ value: 'header', label: 'Custom header' }
	];

	const methodHasBody = $derived(['POST', 'PUT', 'PATCH', 'DELETE'].includes(httpMethod));

	const defaultBrowserScript = `import { test, expect } from '@playwright/test';

test('page loads', async ({ page }) => {
  await page.goto('https://example.com');
  await expect(page).toHaveTitle(/Example/);
});
`;

	function resetForm() {
		name = '';
		checkType = 'http';
		intervalSeconds = '60';
		timeoutSeconds = '30';
		failureThreshold = '2';
		requestTab = 'headers';
		httpUrl = '';
		httpMethod = 'GET';
		headerRows = [];
		authType = 'none';
		authUsername = '';
		authPassword = '';
		authToken = '';
		authHeaderName = '';
		authHeaderValue = '';
		httpBody = '';
		httpBodyContentType = 'application/json';
		followRedirects = true;
		skipTlsVerify = false;
		expectedStatusCodes = '';
		bodyContains = '';
		bodyRegex = '';
		maxResponseTimeMs = '';
		tlsMinDaysRemaining = '';
		tcpHost = '';
		tcpPort = '';
		tcpUseTls = false;
		tcpSendPayload = '';
		tcpExpectContains = '';
		tcpTlsMinDaysRemaining = '';
		browserScript = defaultBrowserScript;
		envRows = [];
		error = '';
	}

	function populateFromCheck(c: SyntheticCheck) {
		resetForm();
		name = c.name;
		checkType = c.checkType;
		intervalSeconds = String(c.intervalSeconds);
		timeoutSeconds = String(c.timeoutSeconds);
		failureThreshold = String(c.failureThreshold);
		const cfg = c.config ?? {};
		if (c.checkType === 'http') {
			httpUrl = cfg.url ?? '';
			httpMethod = (cfg.method || 'GET').toUpperCase();
			headerRows = Object.entries(cfg.headers ?? {}).map(([key, value]) => ({ key, value }));
			authType = cfg.auth?.type || 'none';
			authUsername = cfg.auth?.username ?? '';
			authPassword = cfg.auth?.password ?? '';
			authToken = cfg.auth?.token ?? '';
			authHeaderName = cfg.auth?.headerName ?? '';
			authHeaderValue = cfg.auth?.headerValue ?? '';
			httpBody = cfg.body ?? '';
			httpBodyContentType = cfg.bodyContentType || 'application/json';
			followRedirects = cfg.followRedirects ?? true;
			skipTlsVerify = cfg.skipTlsVerify ?? false;
			expectedStatusCodes = (cfg.assertions?.statusCodes ?? []).join(', ');
			bodyContains = cfg.assertions?.bodyContains ?? '';
			bodyRegex = cfg.assertions?.bodyRegex ?? '';
			maxResponseTimeMs = cfg.assertions?.maxResponseTimeMs
				? String(cfg.assertions.maxResponseTimeMs)
				: '';
			tlsMinDaysRemaining = cfg.assertions?.tlsMinDaysRemaining
				? String(cfg.assertions.tlsMinDaysRemaining)
				: '';
		} else if (c.checkType === 'tcp') {
			tcpHost = cfg.host ?? '';
			tcpPort = cfg.port ? String(cfg.port) : '';
			tcpUseTls = cfg.useTls ?? false;
			tcpSendPayload = cfg.sendPayload ?? '';
			tcpExpectContains = cfg.expectContains ?? '';
			tcpTlsMinDaysRemaining = cfg.tlsMinDaysRemaining ? String(cfg.tlsMinDaysRemaining) : '';
		} else if (c.checkType === 'browser') {
			browserScript = cfg.script ?? '';
			envRows = Object.entries(cfg.env ?? {}).map(([key, value]) => ({ key, value }));
		}
	}

	$effect(() => {
		if (open && check) {
			populateFromCheck(check);
		} else if (open && !check) {
			resetForm();
		}
	});

	function handleOpenChange(value: boolean) {
		open = value;
		if (!value) {
			error = '';
		}
	}

	function rowsToMap(rows: { key: string; value: string }[]): Record<string, string> {
		const map: Record<string, string> = {};
		for (const row of rows) {
			if (row.key.trim()) {
				map[row.key.trim()] = row.value;
			}
		}
		return map;
	}

	function buildConfig(): CheckConfig {
		if (checkType === 'http') {
			const statusCodes = expectedStatusCodes
				.split(',')
				.map((s) => parseInt(s.trim(), 10))
				.filter((n) => !isNaN(n));
			return {
				url: httpUrl.trim(),
				method: httpMethod,
				headers: rowsToMap(headerRows),
				auth: {
					type: authType as 'none' | 'basic' | 'bearer' | 'header',
					username: authUsername,
					password: authPassword,
					token: authToken,
					headerName: authHeaderName,
					headerValue: authHeaderValue
				},
				body: methodHasBody ? httpBody : '',
				bodyContentType: methodHasBody ? httpBodyContentType : '',
				followRedirects,
				skipTlsVerify,
				assertions: {
					statusCodes,
					bodyContains,
					bodyRegex,
					maxResponseTimeMs: parseInt(maxResponseTimeMs, 10) || 0,
					tlsMinDaysRemaining: parseInt(tlsMinDaysRemaining, 10) || 0
				}
			};
		}
		if (checkType === 'tcp') {
			return {
				host: tcpHost.trim(),
				port: parseInt(tcpPort, 10) || 0,
				useTls: tcpUseTls,
				sendPayload: tcpSendPayload,
				expectContains: tcpExpectContains,
				tlsMinDaysRemaining: parseInt(tcpTlsMinDaysRemaining, 10) || 0
			};
		}
		return {
			script: browserScript,
			env: rowsToMap(envRows)
		};
	}

	async function handleSubmit() {
		loading = true;
		error = '';
		try {
			const payload = {
				name: name.trim(),
				checkType,
				enabled: check?.enabled ?? true,
				intervalSeconds: parseInt(intervalSeconds, 10) || 60,
				timeoutSeconds: parseInt(timeoutSeconds, 10) || 30,
				failureThreshold: parseInt(failureThreshold, 10) || 2,
				config: buildConfig()
			};
			if (isEditing && check) {
				await api.put(`/synthetics/checks/${check.id}`, payload, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully updated the Monitor', { position: 'top-center' });
			} else {
				await api.post('/synthetics/checks', payload, {
					projectId: projectsState.currentProjectId ?? undefined
				});
				toast.success('Successfully created the Monitor', { position: 'top-center' });
			}
			open = false;
			onSaved();
		} catch (e) {
			error = e instanceof Error ? e.message : 'Failed to save the monitor';
		} finally {
			loading = false;
		}
	}

	const checkTypeLabel = $derived(
		checkTypeOptions.find((o) => o.value === checkType)?.label || 'Select type'
	);
	const intervalLabel = $derived(
		intervalOptions.find((o) => o.value === intervalSeconds)?.label ||
			`Every ${intervalSeconds} seconds`
	);
	const authLabel = $derived(authOptions.find((o) => o.value === authType)?.label || 'No auth');
	const headerCount = $derived(headerRows.filter((r) => r.key.trim()).length);
</script>

<Sheet.Root {open} onOpenChange={handleOpenChange}>
	<Sheet.Content
		side="right"
		class="flex w-full flex-col gap-0 overflow-y-auto sm:w-[760px] sm:max-w-[min(760px,100vw)]"
	>
		<Sheet.Header class="border-b px-6 pb-4">
			<Sheet.Title>{isEditing ? 'Edit Monitor' : 'New Monitor'}</Sheet.Title>
			<Sheet.Description>
				{isEditing
					? 'Update how this monitor probes your service.'
					: 'Probe a URL, port, or browser flow on a schedule and track its uptime.'}
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
					<Label for="monitor-name">Name</Label>
					<Input
						id="monitor-name"
						bind:value={name}
						placeholder="Marketing site"
						disabled={loading}
					/>
				</div>
				<div class="space-y-2">
					<Label>Type</Label>
					<Select.Root type="single" bind:value={checkType} disabled={loading || isEditing}>
						<Select.Trigger class="w-full">{checkTypeLabel}</Select.Trigger>
						<Select.Content>
							{#each checkTypeOptions as option, __index (__index)}
								<Select.Item value={option.value}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>

			<div class="grid gap-4 sm:grid-cols-3">
				<div class="space-y-2">
					<Label>Interval</Label>
					<Select.Root type="single" bind:value={intervalSeconds} disabled={loading}>
						<Select.Trigger class="w-full">{intervalLabel}</Select.Trigger>
						<Select.Content class="max-h-64 overflow-y-auto">
							{#each intervalOptions as option, __index (__index)}
								<Select.Item value={option.value}>{option.label}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="space-y-2">
					<Label for="monitor-timeout">Timeout (seconds)</Label>
					<Input
						id="monitor-timeout"
						type="number"
						min="1"
						max="120"
						bind:value={timeoutSeconds}
						disabled={loading}
					/>
				</div>
				<div class="space-y-2">
					<Label for="monitor-threshold">Failures before down</Label>
					<Input
						id="monitor-threshold"
						type="number"
						min="1"
						max="10"
						bind:value={failureThreshold}
						disabled={loading}
					/>
				</div>
			</div>

			{#if checkType === 'http'}
				<!-- Postman-style request bar: method and URL joined -->
				<div class="space-y-2">
					<Label>Request</Label>
					<div class="flex">
						<Select.Root type="single" bind:value={httpMethod} disabled={loading}>
							<Select.Trigger
								class="w-[110px] rounded-r-none border-r-0 font-mono text-xs font-semibold"
							>
								{httpMethod}
							</Select.Trigger>
							<Select.Content>
								{#each methodOptions as method, __index (__index)}
									<Select.Item value={method}>{method}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
						<Input
							bind:value={httpUrl}
							placeholder="https://example.com/health"
							class="flex-1 rounded-l-none font-mono text-sm"
							disabled={loading}
						/>
					</div>
				</div>

				<Tabs.Root bind:value={requestTab}>
					<Tabs.List class={underlineTabListClass}>
						<Tabs.Trigger value="headers" class={underlineTabTriggerClass}>
							Headers{headerCount > 0 ? ` (${headerCount})` : ''}
						</Tabs.Trigger>
						<Tabs.Trigger value="auth" class={underlineTabTriggerClass}>
							Auth{authType !== 'none' ? ' •' : ''}
						</Tabs.Trigger>
						<Tabs.Trigger value="body" class={underlineTabTriggerClass}>Body</Tabs.Trigger>
						<Tabs.Trigger value="expected" class={underlineTabTriggerClass}>
							Expected results
						</Tabs.Trigger>
					</Tabs.List>
				</Tabs.Root>

				{#if requestTab === 'headers'}
					<KeyValueRows
						bind:rows={headerRows}
						keyPlaceholder="Header name"
						valuePlaceholder="Value"
						addLabel="Add Header"
						disabled={loading}
					/>
				{:else if requestTab === 'auth'}
					<div class="space-y-3">
						<Select.Root type="single" bind:value={authType} disabled={loading}>
							<Select.Trigger class="w-[220px]">{authLabel}</Select.Trigger>
							<Select.Content>
								{#each authOptions as option, __index (__index)}
									<Select.Item value={option.value}>{option.label}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
						{#if authType === 'basic'}
							<div class="grid grid-cols-2 gap-2">
								<Input bind:value={authUsername} placeholder="Username" disabled={loading} />
								<Input
									bind:value={authPassword}
									type="password"
									placeholder="Password"
									disabled={loading}
								/>
							</div>
						{:else if authType === 'bearer'}
							<Input
								bind:value={authToken}
								type="password"
								placeholder="Token"
								disabled={loading}
							/>
						{:else if authType === 'header'}
							<div class="grid grid-cols-2 gap-2">
								<Input bind:value={authHeaderName} placeholder="X-Api-Key" disabled={loading} />
								<Input
									bind:value={authHeaderValue}
									type="password"
									placeholder="Value"
									disabled={loading}
								/>
							</div>
						{:else}
							<p class="text-sm text-muted-foreground">
								The request is sent without authentication.
							</p>
						{/if}
					</div>
				{:else if requestTab === 'body'}
					{#if methodHasBody}
						<div class="space-y-2">
							<textarea
								bind:value={httpBody}
								spellcheck="false"
								rows="6"
								placeholder={'{\n  "example": true\n}'}
								class="w-full resize-y rounded-md border border-input bg-transparent p-3 font-mono text-xs shadow-xs focus-visible:ring-1 focus-visible:ring-ring focus-visible:outline-none"
								disabled={loading}
							></textarea>
							<div class="flex items-center gap-2">
								<Label for="monitor-content-type" class="text-xs text-muted-foreground"
									>Content-Type</Label
								>
								<Input
									id="monitor-content-type"
									bind:value={httpBodyContentType}
									class="w-[260px] font-mono text-xs"
									disabled={loading}
								/>
							</div>
						</div>
					{:else}
						<p class="text-sm text-muted-foreground">
							{httpMethod} requests are sent without a body. Switch the method to POST, PUT, PATCH, or
							DELETE to add one.
						</p>
					{/if}
				{:else}
					<div class="space-y-4">
						<div class="grid gap-3 sm:grid-cols-2">
							<div class="space-y-1">
								<Label for="monitor-status-codes" class="text-xs text-muted-foreground"
									>Status codes (default: any &lt; 400)</Label
								>
								<Input
									id="monitor-status-codes"
									bind:value={expectedStatusCodes}
									placeholder="200, 204"
									disabled={loading}
								/>
							</div>
							<div class="space-y-1">
								<Label for="monitor-max-time" class="text-xs text-muted-foreground"
									>Max response time (ms)</Label
								>
								<Input
									id="monitor-max-time"
									type="number"
									min="0"
									bind:value={maxResponseTimeMs}
									placeholder="No limit"
									disabled={loading}
								/>
							</div>
							<div class="space-y-1">
								<Label for="monitor-body-contains" class="text-xs text-muted-foreground"
									>Body contains</Label
								>
								<Input id="monitor-body-contains" bind:value={bodyContains} disabled={loading} />
							</div>
							<div class="space-y-1">
								<Label for="monitor-body-regex" class="text-xs text-muted-foreground"
									>Body matches regex</Label
								>
								<Input id="monitor-body-regex" bind:value={bodyRegex} disabled={loading} />
							</div>
							<div class="space-y-1">
								<Label for="monitor-tls-days" class="text-xs text-muted-foreground"
									>Min TLS days remaining</Label
								>
								<Input
									id="monitor-tls-days"
									type="number"
									min="0"
									bind:value={tlsMinDaysRemaining}
									placeholder="Off"
									disabled={loading}
								/>
							</div>
						</div>
						<div class="flex flex-wrap gap-6">
							<label class="flex cursor-pointer items-center gap-2 text-sm">
								<Checkbox
									checked={followRedirects}
									onCheckedChange={(checked) => (followRedirects = checked === true)}
									disabled={loading}
								/>
								Follow redirects
							</label>
							<label class="flex cursor-pointer items-center gap-2 text-sm">
								<Checkbox
									checked={skipTlsVerify}
									onCheckedChange={(checked) => (skipTlsVerify = checked === true)}
									disabled={loading}
								/>
								Skip TLS verification
							</label>
						</div>
					</div>
				{/if}
			{:else if checkType === 'tcp'}
				<div class="space-y-2">
					<Label>Target</Label>
					<div class="flex">
						<span
							class="inline-flex items-center rounded-l-md border border-r-0 border-input px-3 font-mono text-xs font-semibold text-muted-foreground"
						>
							TCP
						</span>
						<Input
							id="monitor-host"
							bind:value={tcpHost}
							placeholder="db.example.com"
							class="flex-1 rounded-none font-mono text-sm"
							disabled={loading}
						/>
						<Input
							id="monitor-port"
							type="number"
							min="1"
							max="65535"
							bind:value={tcpPort}
							placeholder="5432"
							class="w-[110px] rounded-l-none border-l-0 font-mono text-sm"
							disabled={loading}
						/>
					</div>
				</div>
				<div class="grid gap-3 sm:grid-cols-2">
					<div class="space-y-1">
						<Label for="monitor-send" class="text-xs text-muted-foreground"
							>Send payload (optional)</Label
						>
						<Input id="monitor-send" bind:value={tcpSendPayload} disabled={loading} />
					</div>
					<div class="space-y-1">
						<Label for="monitor-expect" class="text-xs text-muted-foreground"
							>Expect response containing</Label
						>
						<Input id="monitor-expect" bind:value={tcpExpectContains} disabled={loading} />
					</div>
					<div class="space-y-1">
						<Label for="monitor-tcp-tls-days" class="text-xs text-muted-foreground"
							>Min TLS days remaining</Label
						>
						<Input
							id="monitor-tcp-tls-days"
							type="number"
							min="0"
							bind:value={tcpTlsMinDaysRemaining}
							placeholder="Off"
							disabled={loading}
						/>
					</div>
				</div>
				<label class="flex cursor-pointer items-center gap-2 text-sm">
					<Checkbox
						checked={tcpUseTls}
						onCheckedChange={(checked) => (tcpUseTls = checked === true)}
						disabled={loading}
					/>
					Use TLS
				</label>
			{:else}
				<div class="space-y-2">
					<Label>Playwright test script</Label>
					<CodeEditor bind:value={browserScript} readOnly={loading} minHeight="300px" />
					<p class="text-xs text-muted-foreground">
						A full @playwright/test spec. It runs in headless Chromium on the server (the :browser
						image) or on a connected runner.
					</p>
				</div>
				<div class="space-y-2">
					<Label>Environment variables (optional)</Label>
					<KeyValueRows
						bind:rows={envRows}
						keyPlaceholder="NAME"
						valuePlaceholder="Value"
						addLabel="Add Variable"
						disabled={loading}
					/>
				</div>
			{/if}
		</form>

		<div class="flex justify-end gap-2 border-t px-6 py-4">
			<Button variant="outline" onclick={() => handleOpenChange(false)} disabled={loading}>
				Cancel
			</Button>
			<Button variant={isEditing ? 'default' : 'success'} onclick={handleSubmit} disabled={loading}>
				{#if isEditing}
					<Check class="mr-2 h-4 w-4" />
					{loading ? 'Updating...' : 'Update Monitor'}
				{:else}
					<Plus class="mr-2 h-4 w-4" />
					{loading ? 'Creating...' : 'New Monitor'}
				{/if}
			</Button>
		</div>
	</Sheet.Content>
</Sheet.Root>
