<script lang="ts">
	import { getSSEContext } from '$lib/sse.svelte';
	import { nextSlide, prevSlide, tapAction, wakeScreen, type TapZone } from '$lib/slideNav';

	// Read per tap: Fader.busy isn't reactive, so a boolean prop would stay false.
	let { isBusy }: { isBusy: () => boolean } = $props();

	const sse = getSSEContext();
	// Unknown screen state counts as on, so the tap navigates.
	const screenOff = $derived(sse.screen ? !sse.screen.on : false);

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
		onclick={() => tap('left')}
	></button>
	<button
		type="button"
		class="flex-1 cursor-default outline-none"
		aria-label="Next photo"
		data-testid="kiosk-tap-next"
		onclick={() => tap('right')}
	></button>
</div>
