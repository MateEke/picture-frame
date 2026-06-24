<script lang="ts">
	import SearchCombobox from './SearchCombobox.svelte';
	import { timezoneOffsetLabel } from '$lib/helpers';

	let {
		value = $bindable(),
		placeholder = 'Browser default'
	}: { value: string; placeholder?: string } = $props();

	// Empty value follows the device; the sentinel item lets the user clear back to it.
	const zones =
		typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : [];
	const items = [
		{ value: '', label: 'Browser default' },
		...zones.map((tz) => ({ value: tz, label: tz }))
	];
</script>

<SearchCombobox
	bind:value
	{placeholder}
	{items}
	inputTestId="setting-timezone"
	searchLabel="Show time zones"
>
	{#snippet itemContent(item)}
		{@const offset = item.value ? timezoneOffsetLabel(item.value) : ''}
		{item.label}
		{#if offset}<span class="text-surface-500 ml-1 text-xs">({offset})</span>{/if}
	{/snippet}
</SearchCombobox>
