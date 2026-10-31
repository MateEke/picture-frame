<script lang="ts">
	import type { DisplayDto } from '$lib/api/types.gen';
	import { PlusIcon, Trash2Icon } from '@lucide/svelte';
	import { eq } from '../utils';
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

	const SCREEN_ACTIONS = [
		{ value: 'off', label: 'Turn off' },
		{ value: 'on', label: 'Turn on' }
	] as const;
	const SCREEN_DAYS = [
		{ value: 'daily', label: 'Every day' },
		{ value: 'weekdays', label: 'Weekdays' },
		{ value: 'weekend', label: 'Weekend' }
	] as const;
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
	<Field
		label="Scheduled brightness"
		help="Use the Surface panel backlight: bright during the day, dim at night. Other displays safely ignore this setting."
		class="md:col-span-2"
		changed={display.brightness_schedule.enabled !== savedDisplay.brightness_schedule.enabled}
		onrevert={() =>
			(display.brightness_schedule.enabled = savedDisplay.brightness_schedule.enabled)}
	>
		<label class="flex items-center gap-2 text-sm">
			<input class="checkbox" type="checkbox" bind:checked={display.brightness_schedule.enabled} />
			Enable daily brightness schedule
		</label>
	</Field>
	{#if display.brightness_schedule.enabled}
		<Field
			label="Bright period"
			help="At this time the panel changes to the brighter level."
			changed={display.brightness_schedule.day_start !==
				savedDisplay.brightness_schedule.day_start ||
				display.brightness_schedule.day_level !== savedDisplay.brightness_schedule.day_level}
			onrevert={() => {
				display.brightness_schedule.day_start = savedDisplay.brightness_schedule.day_start;
				display.brightness_schedule.day_level = savedDisplay.brightness_schedule.day_level;
			}}
		>
			<div class="flex items-center gap-3">
				<input class="input" type="time" bind:value={display.brightness_schedule.day_start} /><input
					class="input w-20"
					type="number"
					min="0"
					max="100"
					bind:value={display.brightness_schedule.day_level}
					aria-label="Day brightness percentage"
				/><span class="text-surface-500-400 text-sm">%</span>
			</div>
		</Field>
		<Field
			label="Dim period"
			help="At this time the panel changes to the dimmer level."
			changed={display.brightness_schedule.night_start !==
				savedDisplay.brightness_schedule.night_start ||
				display.brightness_schedule.night_level !== savedDisplay.brightness_schedule.night_level}
			onrevert={() => {
				display.brightness_schedule.night_start = savedDisplay.brightness_schedule.night_start;
				display.brightness_schedule.night_level = savedDisplay.brightness_schedule.night_level;
			}}
		>
			<div class="flex items-center gap-3">
				<input
					class="input"
					type="time"
					bind:value={display.brightness_schedule.night_start}
				/><input
					class="input w-20"
					type="number"
					min="0"
					max="100"
					bind:value={display.brightness_schedule.night_level}
					aria-label="Night brightness percentage"
				/><span class="text-surface-500-400 text-sm">%</span>
			</div>
		</Field>
	{/if}
	<Field
		label="Screen schedule"
		help="Turn the screen on or off at set times. The screen follows the latest rule that has fired, so an off rule and an on rule make an overnight window. The frame keeps running, so it stays reachable. A touch lights it for the idle-blank time."
		class="md:col-span-2"
		changed={!eq(display.screen_rules, savedDisplay.screen_rules)}
		onrevert={() => (display.screen_rules = $state.snapshot(savedDisplay.screen_rules))}
	>
		<div class="space-y-2" data-testid="screen-rules">
			{#each display.screen_rules as rule, i (i)}
				<div class="flex flex-wrap items-center gap-2">
					<select class="select w-auto" bind:value={rule.action} aria-label="Rule {i + 1} action">
						{#each SCREEN_ACTIONS as a (a.value)}
							<option value={a.value}>{a.label}</option>
						{/each}
					</select>
					<input
						class="input w-auto"
						type="time"
						bind:value={rule.time}
						aria-label="Rule {i + 1} time"
					/>
					<select class="select w-auto" bind:value={rule.days} aria-label="Rule {i + 1} days">
						{#each SCREEN_DAYS as d (d.value)}
							<option value={d.value}>{d.label}</option>
						{/each}
					</select>
					<button
						type="button"
						class="btn-icon"
						aria-label="Remove rule {i + 1}"
						onclick={() => display.screen_rules.splice(i, 1)}
					>
						<Trash2Icon class="size-4" />
					</button>
				</div>
			{/each}
			<button
				type="button"
				class="btn preset-tonal-surface btn-sm"
				data-testid="add-screen-rule"
				onclick={() =>
					display.screen_rules.push({
						time: display.screen_rules.length ? '07:00' : '23:00',
						action: display.screen_rules.length ? 'on' : 'off',
						days: 'daily'
					})}
			>
				<PlusIcon class="size-4" /> Add rule
			</button>
		</div>
	</Field>
</div>
