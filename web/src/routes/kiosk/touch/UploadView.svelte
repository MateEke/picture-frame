<script lang="ts">
	import { WifiIcon } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { getSSEContext } from '$lib/sse.svelte';
	import {
		fetchDeviceInfo,
		fetchImages,
		uploadUrl,
		type DeviceInfo,
		type ImageItem
	} from '$lib/touch';
	import { fi } from './fi';
	import QrCode from './QrCode.svelte';

	const sse = getSSEContext();

	let info = $state<DeviceInfo | null>(null);
	let recent = $state<ImageItem[]>([]);

	onMount(() => {
		fetchDeviceInfo().then((d) => (info = d));
	});

	// New photos pop in here while someone is sending them from a phone.
	$effect(() => {
		void sse.libraryRev;
		fetchImages().then((imgs) => {
			if (imgs) recent = imgs.slice(-8).reverse();
		});
	});

	const url = $derived(info ? uploadUrl(info, location.port) : '');
</script>

<div class="flex h-full flex-col items-center gap-6 overflow-y-auto p-8" data-testid="touch-upload">
	<h1 class="self-start text-4xl font-semibold">{fi.upload.title}</h1>

	{#if info && url}
		<QrCode text={url} size={340} />
		<p
			class="text-center text-3xl font-semibold break-all text-amber-300"
			data-testid="touch-upload-url"
		>
			{url}
		</p>
		{#if info.ssid}
			<p class="flex items-center gap-2 text-xl text-neutral-300">
				<WifiIcon class="size-6" />
				{fi.upload.wifi}: <span class="font-semibold text-neutral-100">{info.ssid}</span>
			</p>
		{/if}
	{:else if info}
		<p class="text-2xl text-red-300">{fi.upload.noAddress}</p>
	{/if}

	<ol class="grid w-full list-none gap-3">
		{#each fi.upload.steps as step, i (i)}
			<li class="flex items-start gap-4 rounded-2xl bg-neutral-900 p-5 text-xl leading-snug">
				<span
					class="flex size-10 shrink-0 items-center justify-center rounded-full bg-amber-400 font-bold text-neutral-950"
					>{i + 1}</span
				>
				{step}
			</li>
		{/each}
	</ol>
	<p class="self-start text-lg text-neutral-400">{fi.upload.password}</p>

	{#if recent.length}
		<section class="w-full">
			<h2 class="mb-3 text-2xl font-semibold">{fi.upload.recent}</h2>
			<div class="grid grid-cols-4 gap-2">
				{#each recent as img (img.name)}
					<img
						src="/thumb/{img.name}"
						alt=""
						loading="lazy"
						decoding="async"
						class="aspect-square w-full rounded-xl bg-neutral-900 object-cover"
					/>
				{/each}
			</div>
		</section>
	{/if}
</div>
