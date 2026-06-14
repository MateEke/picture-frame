<script lang="ts">
	import type { ImagePayload } from '$lib/api/types.gen';
	import { ImageOffIcon, ShuffleIcon, ArrowRightIcon } from '@lucide/svelte';
	import { formatDuration } from '$lib/duration';
	import { fade } from 'svelte/transition';
	import { sineInOut } from 'svelte/easing';

	let {
		image,
		interval,
		shuffle
	}: {
		image: ImagePayload | null;
		interval: string;
		shuffle: boolean;
	} = $props();

	const cadence = $derived(formatDuration(interval, 'Manual'));
</script>

<div class="card bg-surface-100-900 reveal space-y-3 p-4">
	<h2 class="h4">Now playing</h2>
	{#if image}
		<div class="bg-surface-200-800 relative aspect-video w-full overflow-hidden rounded-lg">
			{#key image.name}
				<img
					transition:fade={{ duration: 1000, easing: sineInOut }}
					src="/img/{image.name}"
					alt="Currently on the frame"
					data-testid="now-playing-image"
					class="absolute h-full w-full object-cover"
				/>
			{/key}
		</div>
	{:else}
		<div
			class="bg-surface-200-800 text-surface-500-400 flex aspect-video w-full flex-col items-center justify-center gap-2 rounded-lg"
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
