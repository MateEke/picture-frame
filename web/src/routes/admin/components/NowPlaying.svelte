<script lang="ts">
	import type { ImagePayload } from '$lib/api/types.gen';
	import { ImageOffIcon, ShuffleIcon, ArrowRightIcon } from '@lucide/svelte';
	import { formatDuration } from '$lib/duration';
	import Slide from '$lib/Slide.svelte';
	import { fade } from 'svelte/transition';
	import { sineInOut } from 'svelte/easing';

	let {
		image,
		aspect = null,
		interval,
		shuffle
	}: {
		image: ImagePayload | null;
		aspect?: number | null;
		interval: string;
		shuffle: boolean;
	} = $props();

	const cadence = $derived(formatDuration(interval, 'Manual'));
	const names = $derived(image?.names?.length ? image.names : null);
	const boxAspect = $derived(aspect && aspect > 0 ? aspect : 16 / 9);
	// Cap height so a portrait frame's preview doesn't dominate the dashboard.
	const boxMaxWidth = $derived(`calc(${boxAspect} * 18rem)`);
</script>

<div class="card bg-surface-100-900 reveal space-y-3 p-4">
	<h2 class="h4">Now playing</h2>
	{#if names}
		<div
			class="bg-surface-200-800 relative mx-auto w-full overflow-hidden rounded-lg"
			style:aspect-ratio={boxAspect}
			style:max-width={boxMaxWidth}
		>
			{#key names.join('|')}
				<div class="absolute inset-0" transition:fade={{ duration: 1000, easing: sineInOut }}>
					<Slide images={names} vertical={boxAspect < 1} testId="now-playing-image" />
				</div>
			{/key}
		</div>
	{:else}
		<div
			class="bg-surface-200-800 text-surface-500-400 mx-auto flex w-full flex-col items-center justify-center gap-2 rounded-lg"
			style:aspect-ratio={boxAspect}
			style:max-width={boxMaxWidth}
		>
			<ImageOffIcon class="size-7" />
			<p class="text-sm">Waiting for the slideshow…</p>
		</div>
	{/if}
	<div class="text-surface-500-400 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
		<span class="flex items-center gap-1.5">
			{#if shuffle}
				<ShuffleIcon class="size-4" />Shuffle
			{:else}
				<ArrowRightIcon class="size-4" />In order
			{/if}
		</span>
		<span>Every {cadence}</span>
	</div>
</div>
