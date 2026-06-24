<script lang="ts">
	import { Combobox, Portal, useListCollection } from '@skeletonlabs/skeleton-svelte';
	import { CheckIcon, ChevronDownIcon } from '@lucide/svelte';
	import type { Snippet } from 'svelte';

	type Item = { value: string; label: string };

	let {
		value = $bindable(),
		items: allItems,
		placeholder = 'Select…',
		searchLabel = 'Show options',
		inputTestId,
		itemContent
	}: {
		value: string;
		items: Item[];
		placeholder?: string;
		searchLabel?: string;
		inputTestId?: string;
		itemContent?: Snippet<[Item]>;
	} = $props();

	const labelOf = (v: string): string => allItems.find((i) => i.value === v)?.label ?? v;

	// While open, the input is a free search box; while closed it shows the current
	// value's label. The value commits solely through selection.
	let open = $state(false);
	let typed = $state('');
	const inputText = $derived(open ? typed : labelOf(value));
	const items = $derived.by(() => {
		const q = open ? typed.trim().toLowerCase() : '';
		if (!q) return allItems;
		const filtered = allItems.filter(
			(i) => i.value.toLowerCase().includes(q) || i.label.toLowerCase().includes(q)
		);
		return filtered.length > 0 ? filtered : allItems;
	});

	const collection = $derived(
		useListCollection({
			items,
			itemToString: (item) => item.label,
			itemToValue: (item) => item.value
		})
	);
</script>

<Combobox
	{collection}
	placeholder={open ? 'Search…' : placeholder}
	value={[value]}
	inputValue={inputText}
	onInputValueChange={(e) => (typed = e.inputValue)}
	onValueChange={(e) => (value = e.value[0] ?? value)}
	onOpenChange={(e) => {
		open = e.open;
		typed = '';
	}}
	inputBehavior="autohighlight"
	class="w-full"
>
	<Combobox.Control>
		<Combobox.Input data-testid={inputTestId} />
		<Combobox.Trigger aria-label={searchLabel}>
			<ChevronDownIcon size={18} />
		</Combobox.Trigger>
	</Combobox.Control>
	<Portal>
		<Combobox.Positioner>
			<Combobox.Content class="max-h-60 overflow-y-scroll">
				{#each items as item (item.value)}
					<Combobox.Item {item}>
						<Combobox.ItemText>
							{#if itemContent}{@render itemContent(item)}{:else}{item.label}{/if}
						</Combobox.ItemText>
						<Combobox.ItemIndicator>
							<CheckIcon size={16} />
						</Combobox.ItemIndicator>
					</Combobox.Item>
				{/each}
			</Combobox.Content>
		</Combobox.Positioner>
	</Portal>
</Combobox>
