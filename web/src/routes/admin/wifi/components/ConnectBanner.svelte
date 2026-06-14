<script module lang="ts">
	export type ConnectPhase = 'connecting' | 'success' | 'error' | 'unconfirmed';
	export type ConnectBannerState = {
		ssid: string;
		phase: ConnectPhase;
		message?: string;
		known: boolean;
		startedAt: number;
	};
</script>

<script lang="ts">
	import { WifiIcon, WifiOffIcon, RefreshCwIcon } from '@lucide/svelte';

	let {
		banner,
		hostname,
		ondismiss
	}: { banner: Omit<ConnectBannerState, 'startedAt'>; hostname?: string; ondismiss: () => void } =
		$props();

	function bannerClass(phase: ConnectPhase): string {
		if (phase === 'success') return 'preset-tonal-success';
		if (phase === 'error') return 'preset-tonal-error';
		if (phase === 'connecting') return 'preset-tonal-warning';
		return 'preset-tonal-surface';
	}
</script>

<div
	class="card {bannerClass(banner.phase)} reveal flex items-start gap-3 p-4"
	data-testid="wifi-banner"
>
	{#if banner.phase === 'connecting'}
		<RefreshCwIcon class="mt-0.5 size-5 shrink-0 animate-spin" />
	{:else if banner.phase === 'success'}
		<WifiIcon class="mt-0.5 size-5 shrink-0" />
	{:else}
		<WifiOffIcon class="mt-0.5 size-5 shrink-0" />
	{/if}
	<div class="min-w-0 flex-1">
		{#if banner.phase === 'connecting'}
			<p class="font-medium">Connecting to {banner.ssid}…</p>
			<p class="text-sm opacity-80">This can take up to 30 seconds.</p>
		{:else if banner.phase === 'success'}
			<p class="font-medium">Connected to {banner.ssid}</p>
		{:else if banner.phase === 'error'}
			<p class="font-medium">Couldn't connect to {banner.ssid}</p>
			{#if banner.known}
				<p class="text-sm opacity-80">
					If this network's password changed, forget it below and reconnect.
				</p>
			{:else}
				<p class="text-sm opacity-80">{banner.message ?? 'Check the password and try again.'}</p>
			{/if}
		{:else}
			<p class="font-medium">Still trying to reach {banner.ssid}…</p>
			<p class="text-sm opacity-80">
				If you moved the frame to another network, find it at
				<strong>{hostname}.local</strong>.
			</p>
		{/if}
	</div>
	{#if banner.phase !== 'connecting'}
		<button class="btn btn-sm preset-tonal shrink-0" onclick={ondismiss}>Dismiss</button>
	{/if}
</div>
