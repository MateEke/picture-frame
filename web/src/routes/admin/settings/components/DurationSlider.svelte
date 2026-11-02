<script lang="ts">
	import { Slider } from '@skeletonlabs/skeleton-svelte';
	import { toSeconds, formatDuration, sliderOptions, type DurationScale } from '#lib/duration.js';
	import Field from './Field.svelte';
	import DurationEditor from './DurationEditor.svelte';

	let {
		value = $bindable(''),
		saved,
		scale,
		label,
		testid,
		help,
		zeroLabel = 'Off',
		changed = false,
		disabled = false,
		onrevert
	}: {
		value?: string;
		/** Stored value; stays a slider stop after you move off it. */
		saved?: string;
		scale: DurationScale;
		label: string;
		testid: string;
		help?: string;
		zeroLabel?: string;
		changed?: boolean;
		disabled?: boolean;
		onrevert?: () => void;
	} = $props();

	// What the open editor last wrote; Discard, Revert or a drag changes value and closes it.
	let written = $state<string | null>(null);
	let readoutButton: HTMLButtonElement | undefined = $state();

	const editing = $derived(written === value && !disabled);
	const readout = $derived(formatDuration(value, zeroLabel));
	const options = $derived(sliderOptions(scale.stops, value, saved));
	const index = $derived(options.findIndex((o) => toSeconds(o) === toSeconds(value)));

	function write(next: string) {
		value = written = next;
	}

	function close(returnFocus: boolean) {
		written = null;
		if (returnFocus) readoutButton?.focus();
	}

	function slide(next: string | undefined) {
		value = next ?? value;
		written = null;
	}
</script>

<Field {label} {help} {changed} {onrevert}>
	{#snippet trailing()}
		<button
			bind:this={readoutButton}
			type="button"
			class={[
				'focus-visible:outline-primary-500 -m-1 rounded-sm p-1 text-sm font-medium tabular-nums focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed',
				editing
					? 'text-primary-500'
					: 'hover:text-primary-500 underline decoration-current/40 decoration-dotted underline-offset-4 disabled:no-underline'
			]}
			title="Set a custom value"
			aria-label="{label}: {readout}, edit"
			aria-expanded={editing}
			{disabled}
			onclick={() => (written = value)}
			data-testid="{testid}-value"
		>
			{readout}
		</button>
	{/snippet}
	{#if editing}
		<DurationEditor {value} {scale} {label} {zeroLabel} {testid} onwrite={write} onclose={close} />
	{/if}
	<Slider
		value={[index]}
		min={0}
		max={options.length - 1}
		step={1}
		{disabled}
		onValueChange={(e) => slide(options[e.value[0]])}
	>
		<Slider.Control>
			<Slider.Track><Slider.Range /></Slider.Track>
			<Slider.Thumb index={0} data-testid="{testid}-thumb"><Slider.HiddenInput /></Slider.Thumb>
		</Slider.Control>
	</Slider>
</Field>
