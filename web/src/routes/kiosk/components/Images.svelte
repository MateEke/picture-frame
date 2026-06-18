<script lang="ts">
	import { getSSEContext } from '$lib/sse.svelte';
	import { Fader } from '$lib/fader.svelte';
	import Slide from '$lib/Slide.svelte';
	import { onDestroy, untrack } from 'svelte';

	const sse = getSSEContext();
	const fader = new Fader();

	// The Fader keys on a single opaque src; join the slide's names into one
	// ('|' is safe in filenames) and splitKey reverses it.
	const names = $derived(sse.ready && sse.image?.names?.length ? sse.image.names : null);

	$effect(() => {
		if (!names) return;
		untrack(() => fader.show(names.join('|')));
	});

	function splitKey(key: string): string[] {
		return key ? key.split('|') : [];
	}

	const bottomNames = $derived(splitKey(fader.bottomSrc));
	const topNames = $derived(splitKey(fader.topSrc));

	onDestroy(() => fader.stop());
</script>

<!-- Manual crossfade, not transition:fade: the Pi Zero animates it too choppily. -->
<Slide
	class="fixed top-0 left-0 transform-gpu"
	data-testid="kiosk-slide-bottom"
	images={bottomNames}
	testId="kiosk-img-bottom"
	onAllLoad={() => fader.onBottomLoad()}
	onError={() => fader.onBottomError()}
/>

<Slide
	class={[
		'fixed top-0 left-0 transform-gpu will-change-[opacity]',
		fader.transitioning ? 'transition-opacity duration-3000 ease-in-out' : ''
	]}
	style="opacity: {fader.topOp}"
	ontransitionend={(e) => {
		if (e.propertyName === 'opacity') fader.onTransitionEnd();
	}}
	images={topNames}
	onAllLoad={() => fader.onTopLoad()}
	onError={() => fader.onTopError()}
/>
