<script lang="ts">
	import type { DisplayDto } from '$lib/api/types.gen';
	import Field from './Field.svelte';
	import DeviceCombobox from './DeviceCombobox.svelte';

	let {
		display = $bindable(),
		savedDisplay,
		outputs,
		rotationSupported
	}: {
		display: DisplayDto;
		savedDisplay: DisplayDto;
		outputs: string[];
		rotationSupported: boolean;
	} = $props();

	// wlr-randr transform degrees (counter-clockwise, per Wayland); labels by
	// visible effect, confirmed on-device.
	const ROTATIONS = [
		{ value: 0, label: 'Landscape (0°)' },
		{ value: 90, label: 'Portrait, rotated left (90°)' },
		{ value: 180, label: 'Upside down (180°)' },
		{ value: 270, label: 'Portrait, rotated right (270°)' }
	];
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
		<Field
			label="Rotation"
			help="Physical screen rotation, applied live. In portrait, portrait photos fill the screen and landscape photos pair up."
			class="md:col-span-2"
			changed={display.rotation !== savedDisplay.rotation}
			onrevert={() => (display.rotation = savedDisplay.rotation)}
		>
			<select
				class="select"
				bind:value={display.rotation}
				disabled={!rotationSupported}
				data-testid="setting-rotation"
			>
				{#each ROTATIONS as r (r.value)}
					<option value={r.value}>{r.label}</option>
				{/each}
			</select>
			{#if !rotationSupported}
				<p class="text-surface-500-400 text-xs" data-testid="setting-rotation-hint">
					Screen rotation requires an updated install. Rerun the install script to enable it.
				</p>
			{/if}
		</Field>
	{/if}
</div>
