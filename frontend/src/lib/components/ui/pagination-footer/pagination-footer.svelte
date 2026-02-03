<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';
	import { ChevronLeft, ChevronRight } from 'lucide-svelte';

	let {
		currentPage,
		totalPages,
		pageSize,
		totalItems,
		itemsShown,
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
		itemsShown?: number;
		onPageChange: (page: number) => void;
		onPageSizeChange: (size: number) => void;
		loading?: boolean;
		pageSizeOptions?: number[];
		itemLabel?: string;
	} = $props();

	const selectOptions = $derived(
		pageSizeOptions.map((size) => ({ value: size.toString(), label: size.toString() }))
	);

	const pageSizeLabel = $derived(
		selectOptions.find((o) => o.value === pageSize.toString())?.label ?? pageSize.toString()
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

<div class="flex flex-col items-center justify-between gap-1.5 px-2 sm:hidden">
	<div class="flex w-[100%] justify-between">
		<div class="flex flex-1 items-center text-sm text-muted-foreground">
			{#if itemsShown !== undefined}
				Showing {itemsShown} of {totalItems} {itemLabel}{plural}
			{:else}
				{totalItems} {itemLabel}{plural} total
			{/if}
		</div>

		<div class="flex items-center space-x-2">
			<p class="text-sm font-medium">Rows per page</p>
			<Select.Root type="single" value={pageSize.toString()} onValueChange={handlePageSizeSelect}>
				<Select.Trigger class="h-8 w-[70px]">
					{pageSizeLabel}
				</Select.Trigger>
				<Select.Content side="top">
					{#each selectOptions as option}
						<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
	</div>

	<div class="flex w-[100%] justify-between">
		<div class="flex w-[100px] items-center justify-start text-sm font-medium">
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

<div class="hidden items-center justify-between px-2 sm:flex">
	<div class="flex-1 text-sm text-muted-foreground">
		{#if itemsShown !== undefined}
			Showing {itemsShown} of {totalItems} {itemLabel}{plural}
		{:else}
			{totalItems} {itemLabel}{plural} total
		{/if}
	</div>
	<div class="flex items-center space-x-6 lg:space-x-8">
		<div class="flex items-center space-x-2">
			<p class="text-sm font-medium">Rows per page</p>
			<Select.Root type="single" value={pageSize.toString()} onValueChange={handlePageSizeSelect}>
				<Select.Trigger class="h-8 w-[70px]">
					{pageSizeLabel}
				</Select.Trigger>
				<Select.Content side="top">
					{#each selectOptions as option}
						<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
		<div class="flex w-[100px] items-center justify-center text-sm font-medium">
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
