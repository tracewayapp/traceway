<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import { ChevronLeft, ChevronRight } from '@lucide/svelte';

	let {
		currentPage,
		totalPages,
		pageSize,
		totalItems,
		onPageChange,
		onPageSizeChange,
		loading = false,
		pageSizeOptions = [10, 20, 50, 100],
		itemLabel = 'item'
	}: {
		currentPage: number;
		totalPages: number;
		pageSize: number;
		totalItems: number;
		onPageChange: (page: number) => void;
		onPageSizeChange: (size: number) => void;
		loading?: boolean;
		pageSizeOptions?: number[];
		itemLabel?: string;
	} = $props();

	const selectOptions = $derived(
		[...new Set([...pageSizeOptions, pageSize])]
			.sort((a, b) => a - b)
			.map((size) => ({ value: size.toString(), label: size.toString() }))
	);

	function handlePrevPage() {
		if (currentPage > 1) {
			onPageChange(currentPage - 1);
		}
	}

	function handleNextPage() {
		if (currentPage < totalPages) {
			onPageChange(currentPage + 1);
		}
	}

	function handlePageSizeSelect(value: string | undefined) {
		if (value) {
			onPageSizeChange(parseInt(value));
		}
	}

	const displayedPages = $derived(totalPages || 1);
	const plural = $derived(totalItems === 1 ? '' : 's');
</script>

{#if totalItems > 0}
	<div class="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 px-2">
		<div class="text-sm whitespace-nowrap text-muted-foreground">
			{totalItems}
			{itemLabel}{plural} total
		</div>
		<div class="flex flex-wrap items-center gap-x-6 gap-y-2">
			<div class="flex items-center space-x-2">
				<p class="text-sm font-medium whitespace-nowrap">Rows per page</p>
				<Select.Root type="single" value={pageSize.toString()} onValueChange={handlePageSizeSelect}>
					<Select.Trigger class="h-8 w-[70px]">
						{pageSize}
					</Select.Trigger>
					<Select.Content side="top">
						{#each selectOptions as option (option.value)}
							<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</div>
			<div class="text-sm font-medium whitespace-nowrap">
				Page {currentPage} of {displayedPages}
			</div>
			<div class="flex items-center space-x-2">
				<Button
					variant="outline"
					size="sm"
					class="h-8 w-8 p-0"
					onclick={handlePrevPage}
					disabled={currentPage <= 1 || loading}
				>
					<span class="sr-only">Go to previous page</span>
					<ChevronLeft class="h-4 w-4" />
				</Button>
				<Button
					variant="outline"
					size="sm"
					class="h-8 w-8 p-0"
					onclick={handleNextPage}
					disabled={currentPage >= totalPages || loading}
				>
					<span class="sr-only">Go to next page</span>
					<ChevronRight class="h-4 w-4" />
				</Button>
			</div>
		</div>
	</div>
{/if}
