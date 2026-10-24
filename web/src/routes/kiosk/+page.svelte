<script lang="ts">
	import { Heartbeat } from '$lib/heartbeat';
	import { getSSEContext } from '$lib/sse.svelte';
	import { reloadOnBackendVersionChange } from '$lib/versionReload.svelte';
	import { onMount } from 'svelte';
	import Images from './components/Images.svelte';
	import Overlay from './components/Overlay.svelte';
	import BatteryIndicator from '$lib/BatteryIndicator.svelte';

	const sse = getSSEContext();

	onMount(() => {
		const heartbeat = new Heartbeat();
		heartbeat.start();
		return () => heartbeat.stop();
	});

	// Reload onto the new bundle after a self-update swaps the binary.
	reloadOnBackendVersionChange(() => sse.kiosk?.version);
</script>

{#if sse.ready}
	<div class="h-screen w-screen overflow-hidden">
		<Images />
		<Overlay />
	</div>
	<!-- Battery indicator floating top-right on the picture frame -->
	{#if sse.battery}
		<div class="pointer-events-none fixed top-3 right-4 z-50">
			<BatteryIndicator battery={sse.battery} fgClass="text-kiosk-fg" />
		</div>
	{/if}
{:else}
	<div class="h-screen w-screen overflow-hidden bg-black"></div>
{/if}
