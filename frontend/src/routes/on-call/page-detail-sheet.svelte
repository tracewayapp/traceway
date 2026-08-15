<script lang="ts">
	import * as Sheet from '$lib/components/ui/sheet';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import { Check, CheckCheck, ExternalLink } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { api } from '$lib/api';
	import { projectsState } from '$lib/state/projects.svelte';
	import { type PageDetailResponse, type PageNotification } from '$lib/state/oncall.svelte';
	import { formatDateTime, formatRelativeTimeAgo } from '$lib/utils/formatters';
	import { runPageAction } from './page-actions';
	import PageBadges from './page-badges.svelte';
	import ResolvePageDialog from './resolve-page-dialog.svelte';
	import EscalationChain from '$lib/components/traceway/escalation-chain.svelte';

	interface Props {
		open: boolean;
		pageId: number | null;
		onChanged: () => void;
	}

	let { open = $bindable(), pageId, onChanged }: Props = $props();

	let detail = $state<PageDetailResponse | null>(null);
	let loading = $state(false);
	let bodyExpanded = $state(false);
	let actionLoading = $state(false);

	const pageData = $derived(detail?.page ?? null);

	let loadSeq = 0;

	$effect(() => {
		if (open && pageId !== null) {
			loadDetail(pageId);
		} else if (!open) {
			loadSeq++;
			detail = null;
			bodyExpanded = false;
		}
	});

	async function loadDetail(id: number) {
		const seq = ++loadSeq;
		loading = true;
		try {
			const res = await api.get(`/pages/${id}`, {
				projectId: projectsState.currentProjectId ?? undefined
			});
			if (seq !== loadSeq) return;
			detail = res;
		} catch (e: unknown) {
			if (seq !== loadSeq) return;
			const status = (e as { status?: number }).status;
			toast.error(status === 404 ? 'Page not found' : 'Failed to load page', {
				position: 'top-center'
			});
			open = false;
		} finally {
			if (seq === loadSeq) loading = false;
		}
	}

	function userName(userId: number | null): string {
		if (userId === null) return 'unknown';
		return detail?.users?.[String(userId)] ?? `user #${userId}`;
	}

	function formatTime(value: string | null): string {
		if (!value) return '';
		return formatDateTime(value, { format: 'short' });
	}

	function isScheduled(notification: PageNotification): boolean {
		return (
			notification.status === 'pending' &&
			notification.scheduledFor !== null &&
			new Date(notification.scheduledFor).getTime() > Date.now()
		);
	}

	function formatClock(value: string): string {
		return new Date(value).toLocaleTimeString(undefined, {
			hour: '2-digit',
			minute: '2-digit'
		});
	}

	let resolveDialogOpen = $state(false);

	async function acknowledge() {
		if (!pageData) return;
		actionLoading = true;
		await runPageAction(pageData.id, 'acknowledge');
		actionLoading = false;
		await loadDetail(pageData.id);
		onChanged();
	}

	async function onResolved() {
		if (pageData) {
			await loadDetail(pageData.id);
		}
		onChanged();
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="w-full gap-0 overflow-y-auto sm:max-w-[560px]">
		{#if loading && !detail}
			<div class="flex flex-1 items-center justify-center py-20">
				<LoadingCircle size="xlg" />
			</div>
		{:else if pageData}
			<Sheet.Header class="border-b px-6">
				<Sheet.Title class="pr-8">{pageData.subject || 'Page'}</Sheet.Title>
				<div class="flex flex-wrap items-center gap-2">
					<PageBadges
						severity={pageData.severity}
						urgency={pageData.urgency}
						status={pageData.status}
						longUrgency
					/>
				</div>
			</Sheet.Header>

			<div class="space-y-6 px-6 py-4">
				<div class="grid grid-cols-2 gap-x-4 gap-y-2 text-sm">
					<div>
						<div class="text-xs text-muted-foreground">Rule</div>
						<div>{pageData.ruleName || '—'}</div>
					</div>
					<div>
						<div class="text-xs text-muted-foreground">Rule type</div>
						<div>{pageData.ruleType || '—'}</div>
					</div>
					<div>
						<div class="text-xs text-muted-foreground">Events</div>
						<div>{pageData.eventCount}</div>
					</div>
					<div>
						<div class="text-xs text-muted-foreground">Last event</div>
						<div>{pageData.lastEventAt ? formatRelativeTimeAgo(pageData.lastEventAt) : '—'}</div>
					</div>
				</div>

				{#if pageData.url}
					<a
						href={pageData.url}
						class="inline-flex items-center gap-1 text-sm text-blue-600 hover:underline dark:text-blue-400"
					>
						<ExternalLink class="h-3.5 w-3.5" />
						View in dashboard
					</a>
				{/if}

				{#if pageData.body}
					<div class="space-y-1">
						<div class="text-xs text-muted-foreground">Details</div>
						<p
							class="text-sm whitespace-pre-wrap {bodyExpanded ? '' : 'line-clamp-6'}"
						>{pageData.body}</p>
						{#if pageData.body.length > 300 || pageData.body.split('\n').length > 6}
							<Button
								variant="link"
								size="sm"
								class="h-auto p-0 text-xs"
								onclick={() => (bodyExpanded = !bodyExpanded)}
							>
								{bodyExpanded ? 'Show less' : 'Show more'}
							</Button>
						{/if}
					</div>
				{/if}

				<div class="space-y-2">
					<div class="text-xs text-muted-foreground">Escalation chain</div>
					<EscalationChain
						definition={pageData.policySnapshot}
						currentLevel={pageData.escalationLevel >= 0 ? pageData.escalationLevel : null}
					/>
					{#if pageData.status === 'open' && pageData.nextEscalationAt === null && pageData.escalationLevel >= 0}
						<p class="text-xs text-muted-foreground">
							Escalation exhausted — no further steps will be notified.
						</p>
					{:else if pageData.status === 'open' && pageData.nextEscalationAt}
						<p class="text-xs text-muted-foreground">
							Next escalation at {formatTime(pageData.nextEscalationAt)}
						</p>
					{/if}
				</div>

				<div class="space-y-2">
					<div class="text-xs text-muted-foreground">Delivery timeline</div>
					{#if (detail?.notifications ?? []).length === 0}
						<p class="text-sm text-muted-foreground">No notifications sent yet.</p>
					{:else}
						<div class="space-y-2">
							{#each detail?.notifications ?? [] as notification (notification.id)}
								{@const scheduled = isScheduled(notification)}
								{@const cancelled = notification.status === 'cancelled'}
								<div
									class="flex items-start gap-2 text-sm {scheduled || cancelled
										? 'text-muted-foreground'
										: ''}"
									title={cancelled ? 'Cancelled because the page was acknowledged' : undefined}
								>
									<span
										class="mt-1.5 h-2 w-2 shrink-0 rounded-full {scheduled
											? 'border border-muted-foreground/50 bg-transparent'
											: notification.status === 'sent'
												? 'bg-green-500'
												: notification.status === 'failed'
													? 'bg-destructive'
													: 'bg-muted-foreground/50'}"
									></span>
									<div class="min-w-0 flex-1">
										<div class="flex items-baseline justify-between gap-2">
											<div class="min-w-0">
												<span class="font-medium">L{notification.level + 1}</span>
												<span class="text-muted-foreground">→</span>
												<span class={cancelled ? 'line-through' : ''}>
													{notification.targetDesc}
													{#if !notification.targetDesc.includes(`(${notification.methodType})`)}
														<span class="text-muted-foreground">({notification.methodType})</span>
													{/if}
												</span>
												{#if !scheduled}
													—
													<span
														class={notification.status === 'sent'
															? 'text-green-600 dark:text-green-400'
															: notification.status === 'failed'
																? 'text-destructive'
																: 'text-muted-foreground'}>{notification.status}</span
													>
												{/if}
											</div>
											{#if scheduled && notification.scheduledFor}
												<span class="shrink-0 text-xs text-muted-foreground">
													scheduled for {formatClock(notification.scheduledFor)}
												</span>
											{/if}
										</div>
										{#if notification.status === 'failed' && notification.errorMsg}
											<div class="text-xs break-words text-destructive">{notification.errorMsg}</div>
										{/if}
										{#if !scheduled}
											<div class="text-xs text-muted-foreground">
												{formatTime(notification.sentAt ?? notification.createdAt)}
											</div>
										{/if}
									</div>
								</div>
							{/each}
						</div>
					{/if}
					{#if pageData.acknowledgedAt}
						<div class="flex items-start gap-2 text-sm">
							<span class="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-amber-500"></span>
							<div>
								{#if pageData.acknowledgedVia === 'link'}
									{#if pageData.acknowledgedBy !== null}
										Acknowledged by {userName(pageData.acknowledgedBy)} (via link)
									{:else}
										Acknowledged via ack link
									{/if}
								{:else}
									Acknowledged by {userName(pageData.acknowledgedBy)}
								{/if}
								<div class="text-xs text-muted-foreground">{formatTime(pageData.acknowledgedAt)}</div>
							</div>
						</div>
					{/if}
					{#if pageData.resolvedAt}
						<div class="flex items-start gap-2 text-sm">
							<span class="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-green-500"></span>
							<div>
								Resolved by {userName(pageData.resolvedBy)}
								<div class="text-xs text-muted-foreground">{formatTime(pageData.resolvedAt)}</div>
							</div>
						</div>
					{/if}
				</div>
			</div>

			{#if pageData.status !== 'resolved'}
				<Sheet.Footer class="flex-row justify-end border-t px-6">
					{#if pageData.status === 'open'}
						<Button variant="outline" onclick={acknowledge} disabled={actionLoading}>
							<Check class="mr-1 h-4 w-4" /> Acknowledge
						</Button>
					{/if}
					<Button onclick={() => (resolveDialogOpen = true)} disabled={actionLoading}>
						<CheckCheck class="mr-1 h-4 w-4" /> Resolve
					</Button>
				</Sheet.Footer>
			{/if}
		{/if}
	</Sheet.Content>
</Sheet.Root>

<ResolvePageDialog
	bind:open={resolveDialogOpen}
	pages={pageData ? [pageData] : []}
	onResolved={onResolved}
/>
