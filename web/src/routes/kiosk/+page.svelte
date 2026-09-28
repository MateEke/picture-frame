<script lang="ts">
	import { browser } from '$app/environment';
	import { Heartbeat } from '$lib/heartbeat';
	import { isOnDeviceKiosk } from '$lib/slideNav';
	import { getSSEContext } from '$lib/sse.svelte';
	import { reloadOnBackendVersionChange } from '$lib/versionReload.svelte';
	import { onMount } from 'svelte';
	import Images from './components/Images.svelte';
	import Overlay from './components/Overlay.svelte';
	import TouchApp from './touch/TouchApp.svelte';

	const sse = getSSEContext();
	// The touch menu is for the frame's own screen; a remote viewer only watches.
	const onDevice = browser && isOnDeviceKiosk(location.hostname);

	onMount(() => {
		const heartbeat = new Heartbeat();
		heartbeat.start();
		return () => heartbeat.stop();
	});

	// Reload onto the new bundle after a self-update swaps the binary.
	reloadOnBackendVersionChange(() => sse.kiosk?.version);
</script>

{#if sse.ready && onDevice}
	<TouchApp />
{:else if sse.ready}
	<div class="h-screen w-screen overflow-hidden">
		<Images />
		<Overlay />
	</div>
{:else}
	<div class="h-screen w-screen overflow-hidden bg-black"></div>
{/if}
