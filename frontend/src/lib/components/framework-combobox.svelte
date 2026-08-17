<script lang="ts">
    import { Combobox } from "bits-ui";
    import { cn } from "$lib/utils";
    import { Check, ChevronDown, Search } from "@lucide/svelte";
    import { getFrameworkLabel, type Framework } from '$lib/state/projects.svelte';
    import FrameworkIcon from './framework-icon.svelte';
    import SelectScrollUpButton from '$lib/components/ui/select/select-scroll-up-button.svelte';
    import SelectScrollDownButton from '$lib/components/ui/select/select-scroll-down-button.svelte';

    interface Props {
        value: Framework;
        disabled?: boolean;
    }

    let { value = $bindable(), disabled = false }: Props = $props();

    let open = $state(false);
    let searchQuery = $state('');
    let searchInputRef = $state<HTMLInputElement | null>(null);
    let triggerWrapperRef = $state<HTMLDivElement | null>(null);
    let contentRef = $state<HTMLElement | null>(null);

    const GROUP_ORDER = ['Backend', 'Browser', 'Mobile'] as const;

    const frameworks = [
        { value: 'opentelemetry', label: 'OpenTelemetry', description: 'Any backend: Go, Node.js, Python, PHP, Java, .NET, Ruby', group: 'Backend', keywords: 'otel opentelemetry go golang gin fiber chi fasthttp echo node nodejs express nestjs fastify koa hono cloudflare workers python django flask fastapi php symfony laravel slim java spring dotnet .net csharp ruby rails api server backend' },
        { value: 'react', label: 'React', description: 'React apps, including Next.js and Remix', group: 'Browser', keywords: 'react nextjs next.js remix browser frontend spa' },
        { value: 'svelte', label: 'Svelte', description: 'Svelte and SvelteKit apps', group: 'Browser', keywords: 'svelte sveltekit browser frontend' },
        { value: 'vuejs', label: 'Vue.js', description: 'Vue and Nuxt apps', group: 'Browser', keywords: 'vue vuejs nuxt browser frontend' },
        { value: 'jquery', label: 'jQuery', description: 'Legacy-friendly AJAX and DOM library', group: 'Browser', keywords: 'jquery browser frontend legacy' },
        { value: 'flutter', label: 'Flutter', description: 'Flutter apps on Android and iOS', group: 'Mobile', keywords: 'flutter dart mobile android ios' },
        { value: 'react-native', label: 'React Native', description: 'React Native and Expo apps', group: 'Mobile', keywords: 'react native expo mobile' },
        { value: 'android', label: 'Android', description: 'Native Android (Kotlin/Java)', group: 'Mobile', keywords: 'android kotlin java mobile native' },
        { value: 'ios', label: 'iOS', description: 'Native iOS (Swift/Objective-C)', group: 'Mobile', keywords: 'ios swift swiftui objective-c apple mobile native' },
    ] as const;

    const selectedFrameworkLabel = $derived(getFrameworkLabel(value));

    const query = $derived(searchQuery.toLowerCase().trim());

    const filteredFrameworks = $derived(
        query === ''
            ? frameworks
            : frameworks.filter(f =>
                f.label.toLowerCase().includes(query) ||
                f.description.toLowerCase().includes(query) ||
                f.group.toLowerCase().includes(query) ||
                f.keywords.includes(query)
            )
    );

    const groups = $derived(
        GROUP_ORDER
            .map(name => ({ name, items: filteredFrameworks.filter(f => f.group === name) }))
            .filter(g => g.items.length > 0)
    );

    function handleOpenChange(isOpen: boolean) {
        open = isOpen;
        if (!isOpen) {
            searchQuery = '';
        } else {
            requestAnimationFrame(() => {
                searchInputRef?.focus();
                if (contentRef && triggerWrapperRef) {
                    contentRef.style.width = `${triggerWrapperRef.offsetWidth}px`;
                }
            });
        }
    }
</script>

<Combobox.Root type="single" bind:value bind:open onOpenChange={handleOpenChange}>
    <div class="relative w-full" bind:this={triggerWrapperRef}>
        <Combobox.Input
            {disabled}
            readonly
            defaultValue={selectedFrameworkLabel}
            class="absolute inset-0 w-full h-full opacity-0 pointer-events-none"
            tabindex={-1}
        />
        <Combobox.Trigger
            {disabled}
            data-slot="select-trigger"
            data-size="default"
            class={cn(
                "border-input data-[placeholder]:text-muted-foreground [&_svg:not([class*='text-'])]:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive dark:bg-input/30 dark:hover:bg-input/50 flex w-full items-center justify-between gap-2 rounded-md border bg-transparent px-3 py-2 text-sm whitespace-nowrap shadow-xs transition-[color,box-shadow] outline-none select-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50 data-[size=default]:h-9 data-[size=sm]:h-8 *:data-[slot=select-value]:line-clamp-1 *:data-[slot=select-value]:flex *:data-[slot=select-value]:items-center *:data-[slot=select-value]:gap-2 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
            )}
        >
            <div class="flex items-center gap-2">
                <FrameworkIcon framework={value} />
                <span>{selectedFrameworkLabel}</span>
            </div>
            <ChevronDown class="size-4 opacity-50" />
        </Combobox.Trigger>
    </div>

    <Combobox.Portal>
        <Combobox.Content
            bind:ref={contentRef}
            sideOffset={4}
            preventScroll={true}
            class={cn(
                "bg-popover text-popover-foreground data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-end-2 data-[side=right]:slide-in-from-start-2 data-[side=top]:slide-in-from-bottom-2 relative z-50 min-w-[8rem] rounded-md border shadow-md data-[side=bottom]:translate-y-1 data-[side=left]:-translate-x-1 data-[side=right]:translate-x-1 data-[side=top]:-translate-y-1"
            )}
        >
            <div class="flex items-center border-b px-3 py-2">
                <Search class="mr-2 size-4 shrink-0 opacity-50" />
                <input
                    bind:this={searchInputRef}
                    bind:value={searchQuery}
                    placeholder="Search frameworks..."
                    class="flex h-7 w-full rounded-md bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                />
            </div>

            <SelectScrollUpButton />
            <Combobox.Viewport class="w-full max-h-72 overflow-y-auto scroll-my-1 p-1">
                {#if filteredFrameworks.length === 0}
                    <div class="py-4 text-center text-sm text-muted-foreground">No frameworks found</div>
                {:else}
                    {#each groups as group (group.name)}
                        <Combobox.Group>
                            <Combobox.GroupHeading class="px-2 py-1.5 text-xs font-semibold text-muted-foreground">{group.name}</Combobox.GroupHeading>
                            {#each group.items as fw (fw.value)}
                                <Combobox.Item
                                    value={fw.value}
                                    class={cn(
                                        "data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground [&_svg:not([class*='text-'])]:text-muted-foreground relative flex w-full cursor-default items-center gap-2 rounded-sm py-1.5 ps-2 pe-8 text-sm outline-hidden select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"
                                    )}
                                >
                                    {#snippet children({ selected })}
                                        <div class="flex items-center gap-2">
                                            <FrameworkIcon framework={fw.value} />
                                            <div class="flex flex-col">
                                                <span class="font-medium">{fw.label}</span>
                                                <span class="text-xs text-muted-foreground">{fw.description}</span>
                                            </div>
                                        </div>
                                        {#if selected}
                                            <Check class="absolute end-2 size-4" />
                                        {/if}
                                    {/snippet}
                                </Combobox.Item>
                            {/each}
                        </Combobox.Group>
                    {/each}
                {/if}
            </Combobox.Viewport>
            <SelectScrollDownButton />
        </Combobox.Content>
    </Combobox.Portal>
</Combobox.Root>
