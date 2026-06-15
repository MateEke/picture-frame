<script lang="ts">
	import { RefreshCwIcon } from '@lucide/svelte';
	import { isWPA3Only, groupNetworks } from '$lib/wifi';
	import type { WiFiState, WiFiNetwork } from '$lib/api/types.gen';
	import SignalBars from '$lib/SignalBars.svelte';

	let {
		networks,
		status,
		scanning,
		isConnecting,
		onscan,
		onconnect,
		onforget,
		onjoinhidden
	}: {
		networks: WiFiNetwork[];
		status: WiFiState;
		scanning: boolean;
		isConnecting: boolean;
		onscan: () => void;
		onconnect: (net: WiFiNetwork) => void;
		onforget: (net: WiFiNetwork) => void;
		onjoinhidden: () => void;
	} = $props();

	function isActive(net: WiFiNetwork): boolean {
		return status.mode === 'connected' && net.ssid === status.ssid;
	}

	const activeSSID = $derived(status.mode === 'connected' ? status.ssid : null);
	const grouped = $derived(groupNetworks(networks, activeSSID));
</script>

{#snippet networkRow(net: WiFiNetwork)}
	{const wpa3Only = isWPA3Only(net.security)}
	{const active = $derived(isActive(net))}
	<div
		data-testid="wifi-net"
		class="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg p-3 {wpa3Only
			? 'bg-surface-200-800 opacity-60'
			: 'bg-surface-100-900 hover:bg-surface-200-800'}"
	>
		<SignalBars signal={net.signal} />
		<span class="min-w-0 flex-1 truncate font-medium">{net.ssid}</span>
		<div class="flex w-full shrink-0 flex-wrap items-center justify-end gap-2 sm:w-auto">
			{#if net.security}
				<span class="badge preset-tonal-surface text-xs">{net.security}</span>
			{/if}
			{#if net.hidden}
				<span class="badge preset-tonal-surface text-xs">Hidden</span>
			{/if}
			{#if active}
				<span class="badge preset-tonal-success text-xs">Connected</span>
			{:else if wpa3Only}
				<span
					class="badge preset-tonal-error text-xs"
					title="Pi Zero W radio does not support WPA3-only networks"
				>
					Unsupported
				</span>
			{:else}
				<button
					class="btn preset-tonal-primary btn-sm"
					data-testid="wifi-connect-{net.ssid}"
					disabled={status.mode === 'connecting' || isConnecting}
					onclick={() => onconnect(net)}
				>
					Connect
				</button>
			{/if}
			<!-- Forget is available for any known network, including WPA3-only ones. -->
			{#if net.known}
				<button
					class="btn preset-tonal-error btn-sm"
					data-testid="wifi-forget-{net.ssid}"
					onclick={() => onforget(net)}>Forget</button
				>
			{/if}
		</div>
	</div>
{/snippet}

<div class="card bg-surface-100-900 reveal space-y-4 p-6">
	<div class="flex items-center justify-between">
		<h2 class="h4">Networks</h2>
		<div class="flex items-center gap-2">
			<button
				class="btn preset-tonal-surface btn-sm"
				data-testid="wifi-join-hidden"
				onclick={onjoinhidden}
				disabled={status.mode === 'connecting' || isConnecting}
			>
				Join hidden
			</button>
			<button
				class="btn preset-tonal-surface btn-sm"
				data-testid="wifi-scan"
				onclick={onscan}
				disabled={scanning || status.mode === 'connecting' || isConnecting}
			>
				<RefreshCwIcon class="size-4 {scanning ? 'animate-spin' : ''}" />
				{scanning ? 'Scanning…' : 'Scan'}
			</button>
		</div>
	</div>

	{#if networks.length === 0}
		<p class="text-surface-500-400 text-sm">No networks found. Press Scan to search.</p>
	{:else}
		{#if grouped.saved.length > 0}
			<div class="space-y-2">
				<p class="text-surface-500-400 text-xs font-semibold uppercase">Saved</p>
				{#each grouped.saved as net (net.ssid)}
					{@render networkRow(net)}
				{/each}
			</div>
		{/if}
		{#if grouped.available.length > 0}
			<div class="space-y-2">
				<p class="text-surface-500-400 text-xs font-semibold uppercase">Available</p>
				{#each grouped.available as net (net.ssid)}
					{@render networkRow(net)}
				{/each}
			</div>
		{/if}
	{/if}
</div>
