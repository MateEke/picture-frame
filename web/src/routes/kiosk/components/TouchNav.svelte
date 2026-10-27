<script lang="ts">
	import { onDestroy } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { getSSEContext } from '$lib/sse.svelte';
	import { nextSlide, prevSlide, tapAction, wakeScreen, type TapZone } from '$lib/slideNav';

	// Read per tap: Fader.busy isn't reactive, so a boolean prop would stay false.
	let { isBusy }: { isBusy: () => boolean } = $props();

	const sse = getSSEContext();
	// Unknown screen state counts as on, so the tap navigates.
	const screenOff = $derived(sse.screen ? !sse.screen.on : false);

	// The shadow is deliberately independent of tapAction: a tap during a
	// crossfade is dropped, but the frame should still acknowledge the touch.
	type TapShadow = { id: number; x: number; y: number };
	let shadows = $state<TapShadow[]>([]);
	let nextShadowId = 0;
	const shadowTimers = new SvelteSet<ReturnType<typeof setTimeout>>();

	function showShadow(event: PointerEvent) {
		if (!event.isPrimary) return;

		const shadow = { id: nextShadowId++, x: event.clientX, y: event.clientY };
		shadows.push(shadow);

		const timer = setTimeout(() => {
			shadows = shadows.filter((s) => s.id !== shadow.id);
			shadowTimers.delete(timer);
		}, 560);
		shadowTimers.add(timer);
	}

	onDestroy(() => {
		for (const timer of shadowTimers) clearTimeout(timer);
	});

	function tap(zone: TapZone) {
		const action = tapAction(zone, { busy: isBusy(), screenOff });
		if (action === null) return;
		wakeScreen(); // every accepted tap counts as presence
		if (action === 'next') nextSlide();
		if (action === 'prev') prevSlide();
	}
</script>

<div
	data-testid="kiosk-touch-nav"
	data-screen-off={screenOff ? 'true' : 'false'}
	class="fixed inset-0 flex touch-manipulation select-none [-webkit-tap-highlight-color:transparent]"
>
	<button
		type="button"
		class="flex-1 cursor-default outline-none"
		aria-label="Previous photo"
		data-testid="kiosk-tap-prev"
		onpointerdown={showShadow}
		onclick={() => tap('left')}
	></button>
	<button
		type="button"
		class="flex-1 cursor-default outline-none"
		aria-label="Next photo"
		data-testid="kiosk-tap-next"
		onpointerdown={showShadow}
		onclick={() => tap('right')}
	></button>

	{#each shadows as shadow (shadow.id)}
		<span
			data-testid="kiosk-tap-shadow"
			aria-hidden="true"
			class="kiosk-tap-shadow pointer-events-none fixed h-24 w-24 rounded-full"
			style="left: {shadow.x}px; top: {shadow.y}px"
		></span>
	{/each}
</div>

<style>
	.kiosk-tap-shadow {
		background: radial-gradient(
			circle,
			rgb(0 0 0 / 0.32) 0%,
			rgb(0 0 0 / 0.2) 38%,
			rgb(0 0 0 / 0) 70%
		);
		animation: kiosk-tap-shadow 560ms cubic-bezier(0.16, 1, 0.3, 1) forwards;
	}

	@keyframes kiosk-tap-shadow {
		from {
			opacity: 0;
			transform: translate(-50%, -50%) scale(0.55);
		}
		18% {
			opacity: 1;
		}
		to {
			opacity: 0;
			transform: translate(-50%, -50%) scale(1.35);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.kiosk-tap-shadow {
			animation: none;
			opacity: 1;
		}
	}
</style>
