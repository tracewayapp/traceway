<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { LoadingCircle } from '$lib/components/ui/loading-circle';
	import * as Avatar from '$lib/components/ui/avatar';
	import { CalendarClock, FolderGit2 } from '@lucide/svelte';
	import { oncallState, type OncallUser } from '$lib/state/oncall.svelte';
	import { formatDateTime } from '$lib/utils/formatters';

	interface Props {
		organizationId: number;
		onGoToTeams: () => void;
	}

	let { organizationId, onGoToTeams }: Props = $props();

	$effect(() => {
		oncallState.loadOverview(organizationId);
	});

	const overview = $derived(oncallState.overview);

	function initials(user: OncallUser): string {
		const source = user.name || user.email;
		return source
			.split(/[\s@._-]+/)
			.filter(Boolean)
			.slice(0, 2)
			.map((p) => p[0]!.toUpperCase())
			.join('');
	}

	function teamProjects(teamId: number) {
		return oncallState.teams.find((t) => t.id === teamId)?.projects ?? [];
	}
</script>

{#if oncallState.overviewLoading}
	<div class="flex justify-center py-12"><LoadingCircle size="xlg" /></div>
{:else if overview.length === 0}
	<div
		class="flex flex-col items-center justify-center rounded-md bg-muted py-20 text-center text-muted-foreground"
	>
		<p class="mb-4">Create a team to get started.</p>
		<Button variant="outline" onclick={onGoToTeams}>Go to Teams</Button>
	</div>
{:else}
	<div class="grid gap-4 lg:grid-cols-2">
		{#each overview as entry (entry.team.id)}
			<Card>
				<CardHeader>
					<CardTitle>{entry.team.name}</CardTitle>
					{#if entry.team.description}
						<CardDescription>{entry.team.description}</CardDescription>
					{/if}
				</CardHeader>
				<CardContent class="space-y-4">
					{#if entry.schedules.length === 0}
						<p class="text-sm text-muted-foreground">No schedules yet.</p>
					{:else}
						{#each entry.schedules as schedule (schedule.id)}
							<div class="space-y-2 rounded-md border p-3">
								<div class="flex items-center gap-2 text-sm font-medium">
									<CalendarClock class="h-4 w-4 text-muted-foreground" />
									{schedule.name}
								</div>
								{#if schedule.oncall.length === 0}
									<p class="text-sm text-muted-foreground">No one is on call right now.</p>
								{:else}
									<div class="flex flex-wrap items-center gap-2">
										{#each schedule.oncall as user (user.userId)}
											<div
												class="flex items-center gap-1.5 rounded-full border bg-muted/50 py-0.5 pr-2.5 pl-0.5"
												title={user.email}
											>
												<Avatar.Root class="h-6 w-6">
													<Avatar.Fallback class="text-[10px]">{initials(user)}</Avatar.Fallback>
												</Avatar.Root>
												<span class="text-sm">{user.name || user.email}</span>
											</div>
										{/each}
										{#if schedule.until}
											<span class="text-xs text-muted-foreground">
												until {formatDateTime(schedule.until, { format: 'short' })}
											</span>
										{/if}
									</div>
								{/if}
								{#if schedule.nextUp}
									<p class="text-xs text-muted-foreground">
										Next: {schedule.nextUp.name || schedule.nextUp.email}{schedule.nextAt
											? ` at ${formatDateTime(schedule.nextAt, { format: 'short' })}`
											: ''}
									</p>
								{/if}
							</div>
						{/each}
					{/if}
					{#if teamProjects(entry.team.id).length > 0}
						<div class="flex flex-wrap items-center gap-1.5">
							<FolderGit2 class="h-4 w-4 text-muted-foreground" />
							{#each teamProjects(entry.team.id) as project (project.projectId)}
								<Badge variant="outline">{project.name}</Badge>
							{/each}
						</div>
					{/if}
				</CardContent>
			</Card>
		{/each}
	</div>
{/if}
