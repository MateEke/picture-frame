<script lang="ts">
	import type { SlideshowDto, DisplayDto, SensorDto } from '$lib/api/types.gen';
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import { Undo2Icon, SlidersHorizontalIcon } from '@lucide/svelte';
	import { DURATION_STOPS } from '$lib/duration';
	import { eq, hasMotionSensor } from '../utils';
	import DurationSlider from './DurationSlider.svelte';
	import LocaleCombobox from './LocaleCombobox.svelte';
	import Field from './Field.svelte';

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

	<div class="flex items-center justify-between">
		<span class="label-text flex items-center gap-1.5">
			Shuffle photos
			{#if randomizeChanged}<span class="bg-primary-500 size-1.5 rounded-full" title="Changed"
				></span>{/if}
		</span>
		<div class="flex items-center gap-2">
			{#if randomizeChanged}
				<button
					type="button"
					class="text-surface-500 hover:text-primary-500"
					onclick={() => (slideshow.randomize = savedSlideshow.randomize)}
					aria-label="Revert shuffle photos"
				>
					<Undo2Icon class="size-3.5" />
				</button>
			{/if}
			<Switch
				checked={slideshow.randomize}
				onCheckedChange={({ checked }) => (slideshow.randomize = checked)}
			>
				<Switch.HiddenInput />
				<Switch.Control><Switch.Thumb /></Switch.Control>
			</Switch>
		</div>
	</div>

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
