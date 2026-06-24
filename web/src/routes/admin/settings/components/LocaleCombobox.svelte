<script lang="ts">
	import SearchCombobox from './SearchCombobox.svelte';
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

	const items = LOCALE_TAGS.map((tag) => ({ value: tag, label: labelFor(tag) }));
</script>

<SearchCombobox bind:value {placeholder} {items} searchLabel="Show languages">
	{#snippet itemContent(item)}
		{item.label}
		<span class="text-surface-500 ml-1 font-mono text-xs">{item.value}</span>
	{/snippet}
</SearchCombobox>
