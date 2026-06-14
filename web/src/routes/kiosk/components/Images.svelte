<script lang="ts">
	import { getSSEContext } from '$lib/sse.svelte';
	import { Fader } from '$lib/fader.svelte';
	import { onDestroy, untrack } from 'svelte';

	const sse = getSSEContext();
	const fader = new Fader();

	const sseImageName = $derived(sse.ready && sse.image ? sse.image.name : null);

	$effect(() => {
		if (!sseImageName) return;
		untrack(() => fader.show(`/img/${sseImageName}`));
	});

	onDestroy(() => fader.stop());
</script>

<!-- Manual crossfade, not transition:fade: the Pi Zero animates it too choppily. -->
<img
	src={fader.bottomSrc}
	alt=""
	data-testid="kiosk-img-bottom"
	decoding="async"
	class="will-change-transform"
	onload={() => fader.onBottomLoad()}
	onerror={() => fader.onBottomError()}
/>

<img
	src={fader.topSrc}
	alt=""
	decoding="async"
	class={[
		'will-change-[opacity]',
		fader.transitioning ? 'transition-opacity duration-3000 ease-in-out' : ''
	]}
	style:opacity={fader.topOp}
	onload={() => fader.onTopLoad()}
	onerror={() => fader.onTopError()}
	ontransitionend={(e) => {
		if (e.propertyName === 'opacity') fader.onTransitionEnd();
	}}
/>

<style lang="postcss">
	@reference "tailwindcss";
	img {
		@apply fixed top-0 left-0 h-full w-full transform-gpu object-cover;
	}
</style>
