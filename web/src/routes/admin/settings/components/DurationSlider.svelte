<script lang="ts">
	import { Slider } from '@skeletonlabs/skeleton-svelte';
	import { toSeconds, formatDuration } from '$lib/duration';
	import Field from './Field.svelte';

	let {
		value = $bindable(''),
		stops,
		label,
		help,
		zeroLabel = 'Off',
		changed = false,
		disabled = false,
		onrevert
	}: {
		value?: string;
		stops: readonly string[];
		label: string;
		help?: string;
		zeroLabel?: string;
		changed?: boolean;
		disabled?: boolean;
		onrevert?: () => void;
	} = $props();

	// Merge the loaded value into the stops if it isn't already one, so a custom
	// config value is never silently snapped away or lost.
	const options = $derived(
		[...(stops.includes(value) ? stops : [...stops, value])].sort(
			(a, b) => toSeconds(a) - toSeconds(b)
		)
	);
	const index = $derived(Math.max(0, options.indexOf(value)));
</script>

<Field {label} {help} {changed} {onrevert}>
	{#snippet trailing()}
		<span class="text-sm font-medium tabular-nums">{formatDuration(value, zeroLabel)}</span>
	{/snippet}
	<Slider
		value={[index]}
		min={0}
		max={options.length - 1}
		step={1}
		{disabled}
		onValueChange={(e) => (value = options[e.value[0]] ?? value)}
	>
		<Slider.Control>
			<Slider.Track><Slider.Range /></Slider.Track>
			<Slider.Thumb index={0}><Slider.HiddenInput /></Slider.Thumb>
		</Slider.Control>
	</Slider>
</Field>
