<script lang="ts">
	import { getSSEContext } from '$lib/sse.svelte';
	import { nextSlide, prevSlide, wakeScreen } from '$lib/slideNav';
	import { classifyGesture } from '$lib/touch';

	// Read per gesture: Fader.busy isn't reactive, so a boolean prop would stay false.
	let { isBusy, onTap }: { isBusy: () => boolean; onTap?: () => void } = $props();

	const sse = getSSEContext();
	// Unknown screen state counts as on, so the tap navigates.
	const screenOff = $derived(sse.screen ? !sse.screen.on : false);

	let start: { x: number; y: number; t: number; id: number } | null = null;

	function down(e: PointerEvent) {
		if (!e.isPrimary) return;
		start = { x: e.clientX, y: e.clientY, t: performance.now(), id: e.pointerId };
	}

	function up(e: PointerEvent) {
		if (!start || e.pointerId !== start.id) return;
		const gesture = classifyGesture(
			e.clientX - start.x,
			e.clientY - start.y,
			performance.now() - start.t
		);
		start = null;
		if (gesture === null) return;
		wakeScreen(); // every accepted gesture counts as presence
		// A dark panel only wakes: nobody meant to open the menu or flip a photo they can't see.
		if (screenOff) return;
		if (gesture === 'tap') {
			onTap?.();
			return;
		}
		if (isBusy()) return;
		if (gesture === 'next') nextSlide();
		if (gesture === 'prev') prevSlide();
	}
</script>

<!-- Tap opens the menu; swipe left/right changes the photo. -->
<div
	data-testid="kiosk-touch-nav"
	data-screen-off={screenOff ? 'true' : 'false'}
	role="presentation"
	class="fixed inset-0 touch-none select-none [-webkit-tap-highlight-color:transparent]"
	onpointerdown={down}
	onpointerup={up}
	onpointercancel={() => (start = null)}
></div>
