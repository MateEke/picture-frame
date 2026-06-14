<script lang="ts">
	import type { Component } from 'svelte';
	import { WifiIcon, WifiOffIcon, RadioIcon, LoaderIcon } from '@lucide/svelte';
	import type { WiFiState } from '$lib/api/types.gen';
	import SignalBars from '$lib/SignalBars.svelte';

	let { status }: { status: WiFiState } = $props();

	type Tone = 'success' | 'primary' | 'warning' | 'surface';
	const toneClass: Record<Tone, string> = {
		success: 'preset-tonal-success',
		primary: 'preset-tonal-primary',
		warning: 'preset-tonal-warning',
		surface: 'preset-tonal-surface'
	};

	interface View {
		icon: Component;
		tone: Tone;
		spin: boolean;
		headline: string;
		detail: string;
	}

	const view = $derived.by((): View => {
		switch (status.mode) {
			case 'connected':
				return {
					icon: WifiIcon,
					tone: 'success',
					spin: false,
					headline: status.ssid,
					detail: status.security ? `${status.security} secured` : 'Open network'
				};
			case 'ap':
				return {
					icon: RadioIcon,
					tone: 'primary',
					spin: false,
					headline: status.ssid,
					detail: 'Hotspot active · join it to set up WiFi'
				};
			case 'connecting':
				return {
					icon: LoaderIcon,
					tone: 'warning',
					spin: true,
					headline: status.ssid,
					detail: 'Connecting…'
				};
			default:
				return {
					icon: WifiOffIcon,
					tone: 'surface',
					spin: false,
					headline: 'Not connected',
					detail: status.ap_enabled ? 'Starting hotspot…' : 'No saved network in range'
				};
		}
	});

	const Icon = $derived(view.icon);
</script>

<div class="card bg-surface-100-900 reveal space-y-4 p-6" data-testid="wifi-status">
	<div class="flex items-center gap-4">
		<div class="grid size-12 shrink-0 place-items-center rounded-full {toneClass[view.tone]}">
			<Icon class="size-6 {view.spin ? 'animate-spin' : ''}" />
		</div>
		<div class="min-w-0 flex-1">
			<p class="truncate text-lg font-semibold">{view.headline}</p>
			<p class="text-surface-500-400 text-sm">{view.detail}</p>
		</div>
		{#if status.mode === 'connected'}
			<span class="flex items-center gap-1.5">
				<SignalBars signal={status.signal} />
				<span class="text-surface-500-400 text-sm tabular-nums">{status.signal}%</span>
			</span>
		{/if}
	</div>

	{#if status.mode === 'connected' && status.hostname}
		<p class="border-surface-200-800 text-surface-500-400 border-t pt-4 text-sm">
			Reachable at
			<span class="text-surface-700-300 font-medium">{status.hostname}.local</span>
			{#if status.ip}<span class="font-mono text-xs">· {status.ip}</span>{/if}
		</p>
	{/if}
</div>
