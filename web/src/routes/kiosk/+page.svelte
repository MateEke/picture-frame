<script lang="ts">
	import { Heartbeat } from '$lib/heartbeat';
	import { getSSEContext } from '$lib/sse.svelte';
	import { reloadOnBackendVersionChange } from '$lib/versionReload.svelte';
	import { onMount } from 'svelte';
	import Images from './components/Images.svelte';
	import Overlay from './components/Overlay.svelte';

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
{:else}
	<div class="h-screen w-screen overflow-hidden bg-black"></div>
{/if}
