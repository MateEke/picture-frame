<script lang="ts">
	import type { SlideshowDto, DisplayDto, SensorDto } from '$lib/api/types.gen';
	import { SlidersHorizontalIcon } from '@lucide/svelte';
	import { DURATION_STOPS } from '$lib/duration';
	import { eq, hasMotionSensor } from '../utils';
	import DurationSlider from './DurationSlider.svelte';
	import LocaleCombobox from './LocaleCombobox.svelte';
	import Field from './Field.svelte';
	import ToggleRow from './ToggleRow.svelte';

	let {
		slideshow = $bindable(),
		display = $bindable(),
		savedSlideshow,
		savedDisplay,
		sensors
	}: {
		slideshow: SlideshowDto;
		display: DisplayDto;
		savedSlideshow: SlideshowDto;
		savedDisplay: DisplayDto;
		sensors: SensorDto[] | null;
	} = $props();

	const motion = $derived(hasMotionSensor(sensors));
	const randomizeChanged = $derived(slideshow.randomize !== savedSlideshow.randomize);
	const splitChanged = $derived(slideshow.split_screen !== savedSlideshow.split_screen);
	// structural compare so a new label field is covered without touching this
	const labelsChanged = $derived(!eq(display.labels, savedDisplay.labels));
</script>

<div class="card bg-surface-100-900 space-y-5 p-6">
	<h2 class="h4 flex items-center gap-2">
		<SlidersHorizontalIcon class="text-primary-500 size-5" /> Essentials
	</h2>

	<DurationSlider
		label="Advance photo every"
		stops={DURATION_STOPS.slideshowInterval}
		bind:value={slideshow.interval}
		changed={slideshow.interval !== savedSlideshow.interval}
		onrevert={() => (slideshow.interval = savedSlideshow.interval)}
	/>

	<ToggleRow
		label="Shuffle photos"
		checked={slideshow.randomize}
		changed={randomizeChanged}
		onchange={(v) => (slideshow.randomize = v)}
		onrevert={() => (slideshow.randomize = savedSlideshow.randomize)}
	/>

	<ToggleRow
		label="Split-screen pairing"
		checked={slideshow.split_screen}
		changed={splitChanged}
		onchange={(v) => (slideshow.split_screen = v)}
		onrevert={() => (slideshow.split_screen = savedSlideshow.split_screen)}
		testId="split-screen-switch"
	/>

	<DurationSlider
		label="Turn screen off when idle"
		help="A motion sensor wakes the screen after it blanks."
		stops={DURATION_STOPS.blankAfter}
		zeroLabel="Never"
		disabled={!motion}
		bind:value={display.blank_after}
		changed={display.blank_after !== savedDisplay.blank_after}
		onrevert={() => (display.blank_after = savedDisplay.blank_after)}
	/>
	{#if !motion}
		<p class="text-surface-500-400 -mt-3 text-xs">
			Add a motion sensor to enable idle blanking. Without one, the screen couldn't wake again.
		</p>
	{/if}

	<Field
		label="Language"
		help="Date and clock format on the frame."
		changed={display.locale !== savedDisplay.locale}
		onrevert={() => (display.locale = savedDisplay.locale)}
	>
		<LocaleCombobox bind:value={display.locale} />
	</Field>

	<Field
		label="Reading labels"
		help="Captions under the readings on the frame, in your own words. Leave one empty to hide it."
		changed={labelsChanged}
		onrevert={() => (display.labels = { ...savedDisplay.labels })}
	>
		<div class="grid grid-cols-3 gap-2">
			<input
				class="input"
				type="text"
				bind:value={display.labels.outside}
				placeholder="Outside"
				aria-label="Outside reading label"
				data-testid="setting-label-outside"
			/>
			<input
				class="input"
				type="text"
				bind:value={display.labels.inside}
				placeholder="Inside"
				aria-label="Inside reading label"
			/>
			<input
				class="input"
				type="text"
				bind:value={display.labels.humidity}
				placeholder="Humidity"
				aria-label="Humidity reading label"
			/>
		</div>
	</Field>
</div>
