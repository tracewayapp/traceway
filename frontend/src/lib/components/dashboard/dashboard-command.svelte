<script lang="ts">
	import * as Command from '$lib/components/ui/command';
	import { api } from '$lib/api';
	import { goto } from '$app/navigation';
	import { addStickyParamsToHref } from '$lib/utils/navigation';
	import { page } from '$app/state';
	import { projectsState } from '$lib/state/projects.svelte';
	import { authState } from '$lib/state/auth.svelte';
	import { toast } from 'svelte-sonner';
	import {
		Plus,
		Upload,
		FolderInput,
		LayoutDashboard,
		Store,
		ChartLine,
		Check,
		Building2
	} from 'lucide-svelte';

	type LibraryDashboard = {
		id: number;
		name: string;
		description: string;
		templateKey: string | null;
		widgetCount: number;
		appliedProjectIds: string[];
	};

	type LibraryOrganization = {
		id: number;
		name: string;
		role: string;
		dashboards: LibraryDashboard[];
	};

	type Template = {
		key: string;
		name: string;
		description: string;
		category: string;
		widgetCount: number;
	};

	type OrgMetric = {
		name: string;
		unit?: string;
		projectIds: string[];
	};

	let {
		open = $bindable(false),
		onCreateNew,
		onImportJson,
		onImportGrafana,
		onApplied,
		onAddMetric
	} = $props<{
		open: boolean;
		onCreateNew?: () => void;
		onImportJson?: () => void;
		onImportGrafana?: () => void;
		onApplied?: (dashboardId: number) => void;
		onAddMetric?: (metricName: string) => void;
	}>();

	let organizations = $state<LibraryOrganization[]>([]);
	let templates = $state<Template[]>([]);
	let metrics = $state<OrgMetric[]>([]);
	let metricsTruncated = $state(false);
	let loaded = $state(false);
	let busy = $state(false);

	const currentProjectId = $derived(projectsState.currentProjectId);
	const currentOrgId = $derived(projectsState.currentProject?.organizationId ?? null);
	const multiOrg = $derived(authState.organizations.length > 1);

	$effect(() => {
		if (open && !loaded) {
			loadData();
		}
		if (!open) {
			loaded = false;
		}
	});

	async function loadData() {
		loaded = true;
		const projectId = currentProjectId ?? undefined;
		const [library, templateList, orgMetrics] = await Promise.all([
			api.get('/dashboards/library', { projectId }).catch(() => ({ organizations: [] })),
			api.get('/dashboard-templates', { projectId }).catch(() => ({ templates: [] })),
			currentOrgId
				? api.get(`/metrics/discover/org?organizationId=${currentOrgId}`, { projectId }).catch(
						() => ({ metrics: [], truncated: false })
					)
				: Promise.resolve({ metrics: [], truncated: false })
		]);
		organizations = library.organizations ?? [];
		templates = templateList.templates ?? [];
		metrics = orgMetrics.metrics ?? [];
		metricsTruncated = !!orgMetrics.truncated;
	}

	function close() {
		open = false;
	}

	function gotoDashboards(params: string) {
		const target = addStickyParamsToHref(`/dashboards${params}`, 'preset', 'from', 'to');
		const current = page.url.pathname + page.url.search;
		if (current === target) return;
		goto(target);
	}

	function runAction(action: 'create' | 'import' | 'grafana') {
		close();
		if (action === 'create' && onCreateNew) return onCreateNew();
		if (action === 'import' && onImportJson) return onImportJson();
		if (action === 'grafana' && onImportGrafana) return onImportGrafana();
		gotoDashboards(`?action=${action}`);
	}

	function finishWithDashboard(dashboardId: number) {
		close();
		if (onApplied) {
			onApplied(dashboardId);
		} else {
			gotoDashboards(`?dashboard=${dashboardId}`);
		}
	}

	async function selectDashboard(org: LibraryOrganization, dashboard: LibraryDashboard) {
		if (!currentProjectId) {
			toast.error('Select a project first', { position: 'top-center' });
			return;
		}
		if (dashboard.appliedProjectIds.includes(currentProjectId)) {
			finishWithDashboard(dashboard.id);
			return;
		}
		busy = true;
		try {
			if (currentOrgId !== null && org.id !== currentOrgId) {
				const copy = await api.post(
					`/dashboards/${dashboard.id}/copy`,
					{ organizationId: currentOrgId, applyToProjectIds: [currentProjectId] },
					{ projectId: currentProjectId }
				);
				toast.success('Successfully copied the Dashboard', { position: 'top-center' });
				finishWithDashboard(copy.id);
			} else {
				const current = await api.get(`/dashboards/${dashboard.id}`, {
					projectId: currentProjectId
				});
				const appliedNow: string[] = current.appliedProjectIds ?? dashboard.appliedProjectIds;
				if (!appliedNow.includes(currentProjectId)) {
					await api.put(
						`/dashboards/${dashboard.id}/apply`,
						{ projectIds: [...appliedNow, currentProjectId] },
						{ projectId: currentProjectId }
					);
				}
				toast.success('Successfully applied the Dashboard', { position: 'top-center' });
				finishWithDashboard(dashboard.id);
			}
		} catch (e: any) {
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to add the dashboard', { position: 'top-center' });
			}
		} finally {
			busy = false;
		}
	}

	async function installTemplate(template: Template) {
		if (!currentProjectId) {
			toast.error('Select a project first', { position: 'top-center' });
			return;
		}
		busy = true;
		try {
			const dashboard = await api.post(
				`/dashboard-templates/${template.key}/install`,
				{},
				{ projectId: currentProjectId }
			);
			toast.success('Successfully installed the Dashboard', { position: 'top-center' });
			finishWithDashboard(dashboard.id);
		} catch (e: any) {
			if (e?.status !== 403) {
				toast.error(e?.message || 'Failed to install the template', { position: 'top-center' });
			}
		} finally {
			busy = false;
		}
	}

	function selectMetric(metric: OrgMetric) {
		close();
		if (onAddMetric) {
			onAddMetric(metric.name);
		} else {
			gotoDashboards(`?addMetric=${encodeURIComponent(metric.name)}`);
		}
	}
</script>

<Command.Dialog
	bind:open
	title="Add Dashboard"
	description="Search dashboards, templates and metrics"
>
	<Command.Input placeholder="Search dashboards, templates, metrics..." />
	<Command.List>
		<Command.Empty>No results found.</Command.Empty>
		<Command.Group heading="Actions">
			<Command.Item value="action-new-dashboard" keywords={['new', 'create', 'dashboard']} onSelect={() => runAction('create')}>
				<Plus />
				<span>New Dashboard</span>
			</Command.Item>
			<Command.Item value="action-import-json" keywords={['import', 'json', 'file']} onSelect={() => runAction('import')}>
				<Upload />
				<span>Import JSON</span>
			</Command.Item>
			<Command.Item value="action-import-grafana" keywords={['import', 'grafana', 'convert']} onSelect={() => runAction('grafana')}>
				<FolderInput />
				<span>Import from Grafana</span>
			</Command.Item>
		</Command.Group>
		{#each organizations as org (org.id)}
			{#if org.dashboards.length > 0}
				<Command.Group heading={multiOrg ? `Dashboards: ${org.name}` : 'Your Dashboards'}>
					{#each org.dashboards as dashboard (dashboard.id)}
						{@const applied = currentProjectId
							? dashboard.appliedProjectIds.includes(currentProjectId)
							: false}
						<Command.Item
							value={`dashboard-${org.id}-${dashboard.id}`}
							keywords={[dashboard.name, dashboard.description, org.name]}
							disabled={busy}
							onSelect={() => selectDashboard(org, dashboard)}
						>
							<LayoutDashboard />
							<div class="flex min-w-0 flex-1 flex-col">
								<span class="truncate">{dashboard.name}</span>
								<span class="truncate text-xs text-muted-foreground">
									{dashboard.widgetCount} widget{dashboard.widgetCount === 1 ? '' : 's'}
									{#if dashboard.description}&middot; {dashboard.description}{/if}
								</span>
							</div>
							{#if currentOrgId !== null && org.id !== currentOrgId}
								<span class="flex items-center gap-1 text-xs text-muted-foreground">
									<Building2 class="h-3 w-3" /> will copy
								</span>
							{:else if applied}
								<span class="flex items-center gap-1 text-xs text-muted-foreground">
									<Check class="h-3 w-3" /> applied
								</span>
							{/if}
						</Command.Item>
					{/each}
				</Command.Group>
			{/if}
		{/each}
		{#if templates.length > 0}
			<Command.Group heading="Templates">
				{#each templates as template (template.key)}
					<Command.Item
						value={`template-${template.key}`}
						keywords={[template.name, template.description, template.category, 'template']}
						disabled={busy}
						onSelect={() => installTemplate(template)}
					>
						<Store />
						<div class="flex min-w-0 flex-1 flex-col">
							<span class="truncate">{template.name}</span>
							<span class="truncate text-xs text-muted-foreground">
								{template.category} &middot; {template.widgetCount} widget{template.widgetCount === 1
									? ''
									: 's'}
								{#if template.description}&middot; {template.description}{/if}
							</span>
						</div>
					</Command.Item>
				{/each}
			</Command.Group>
		{/if}
		{#if metrics.length > 0}
			<Command.Group heading="Metrics">
				{#each metrics as metric (metric.name)}
					<Command.Item
						value={`metric-${metric.name}`}
						keywords={[metric.name, 'metric', 'chart']}
						disabled={busy}
						onSelect={() => selectMetric(metric)}
					>
						<ChartLine />
						<div class="flex min-w-0 flex-1 flex-col">
							<span class="truncate">{metric.name}</span>
							<span class="truncate text-xs text-muted-foreground">
								Add as a widget
								{#if metric.projectIds.length > 1}
									&middot; seen in {metric.projectIds.length} projects
								{/if}
							</span>
						</div>
					</Command.Item>
				{/each}
			</Command.Group>
		{/if}
	</Command.List>
	{#if metricsTruncated && metrics.length > 0}
		<div class="border-t px-3 py-2 text-xs text-muted-foreground">
			Metrics from your 25 most recent projects
		</div>
	{/if}
</Command.Dialog>
