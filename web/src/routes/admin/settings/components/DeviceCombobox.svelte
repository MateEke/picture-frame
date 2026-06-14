<script lang="ts">
	import { Combobox, Portal, useListCollection } from '@skeletonlabs/skeleton-svelte';
	import { CheckIcon, ChevronDownIcon } from '@lucide/svelte';

	let {
		value = $bindable(),
		options,
		placeholder = 'Select or type a device'
	}: { value: string; options: string[]; placeholder?: string } = $props();

	// Detected devices plus the configured value, so a device that isn't currently
	// present (unplugged, or set before this hardware was attached) is never dropped.
	const baseOptions = $derived(
		[...new Set([...options, value].filter((v) => v.length > 0))].sort()
	);

	// While open the input is a free search box; while closed it shows the committed
	// value. Selecting commits via onValueChange; typing a custom value commits on close.
	let open = $state(false);
	let typed = $state('');
	const items = $derived.by(() => {
		const q = open ? typed.trim().toLowerCase() : '';
		if (!q) return baseOptions;
		return baseOptions.filter((o) => o.toLowerCase().includes(q));
	});

	const collection = $derived(
		useListCollection({
			items,
			itemToString: (item) => item,
			itemToValue: (item) => item
		})
	);
</script>

<Combobox
	{collection}
	{placeholder}
	value={value ? [value] : []}
	inputValue={open ? typed : value}
	onInputValueChange={(e) => (typed = e.inputValue)}
	onValueChange={(e) => (value = e.value[0] ?? value)}
	onOpenChange={(e) => {
		open = e.open;
		// Commit a typed custom value that wasn't picked from the list.
		const trimmed = typed.trim();
		if (!e.open && trimmed.length > 0) value = trimmed;
		typed = '';
	}}
	allowCustomValue
	inputBehavior="autohighlight"
	class="w-full"
>
	<Combobox.Control>
		<Combobox.Input />
		<Combobox.Trigger aria-label="Show devices">
			<ChevronDownIcon size={18} />
		</Combobox.Trigger>
	</Combobox.Control>
	<Portal>
		<Combobox.Positioner>
			<Combobox.Content class="max-h-60 overflow-y-auto">
				{#each items as item (item)}
					<Combobox.Item {item}>
						<Combobox.ItemText>{item}</Combobox.ItemText>
						<Combobox.ItemIndicator>
							<CheckIcon size={16} />
						</Combobox.ItemIndicator>
					</Combobox.Item>
				{/each}
			</Combobox.Content>
		</Combobox.Positioner>
	</Portal>
</Combobox>
