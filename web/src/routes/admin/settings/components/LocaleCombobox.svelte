<script lang="ts">
	import { Combobox, Portal, useListCollection } from '@skeletonlabs/skeleton-svelte';
	import { CheckIcon, ChevronDownIcon } from '@lucide/svelte';
	import LOCALE_TAGS from '$lib/locales.json';

	let {
		value = $bindable(),
		placeholder = 'Select a language'
	}: { value: string; placeholder?: string } = $props();

	const labeller = new Intl.DisplayNames(undefined, { type: 'language' });
	const labelFor = (tag: string): string => {
		try {
			return labeller.of(tag) ?? tag;
		} catch {
			return tag;
		}
	};

	const allItems = LOCALE_TAGS.map((tag) => ({ value: tag, label: labelFor(tag) }));

	// While open, the input is a free search box; while closed it shows the current
	// value's human label. The value commits solely through selection, since the
	// locale list is exhaustive and free text is never a valid choice.
	let open = $state(false);
	let typed = $state('');
	const inputText = $derived(open ? typed : labelFor(value));
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
		<Combobox.Input />
		<Combobox.Trigger aria-label="Show languages">
			<ChevronDownIcon size={18} />
		</Combobox.Trigger>
	</Combobox.Control>
	<Portal>
		<Combobox.Positioner>
			<Combobox.Content class="max-h-60 overflow-y-scroll">
				{#each items as item (item.value)}
					<Combobox.Item {item}>
						<Combobox.ItemText>
							{item.label}
							<span class="text-surface-500 ml-1 font-mono text-xs">{item.value}</span>
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
