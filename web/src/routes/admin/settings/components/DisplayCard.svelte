<script lang="ts">
	import type { DisplayDto } from '$lib/api/types.gen';
	import Field from './Field.svelte';
	import DeviceCombobox from './DeviceCombobox.svelte';

	let {
		display = $bindable(),
		savedDisplay,
		outputs
	}: { display: DisplayDto; savedDisplay: DisplayDto; outputs: string[] } = $props();
</script>

<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
	<Field
		label="Backend"
		help="wlopm (default) toggles the panel via the compositor. vcgencmd is the legacy path."
		changed={display.backend !== savedDisplay.backend}
		onrevert={() => (display.backend = savedDisplay.backend)}
	>
		<select class="select" bind:value={display.backend}>
			{#each ['wlopm', 'vcgencmd'] as b (b)}
				<option value={b}>{b}</option>
			{/each}
		</select>
	</Field>
	{#if display.backend === 'wlopm'}
		<Field
			label="Wayland output"
			help="Display connector name, e.g. HDMI-A-1. Pick a connected display or type one."
			class="md:col-span-2"
			changed={display.output !== savedDisplay.output}
			onrevert={() => (display.output = savedDisplay.output)}
		>
			<DeviceCombobox bind:value={display.output} options={outputs} placeholder="HDMI-A-1" />
		</Field>
	{/if}
</div>
