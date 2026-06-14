<script lang="ts">
	import type { WiFiNetwork } from '$lib/api/types.gen';
	import PasswordInput from '$lib/PasswordInput.svelte';

	let {
		network,
		onconnect,
		oncancel
	}: {
		network: WiFiNetwork;
		onconnect: (ssid: string, password: string) => void;
		oncancel: () => void;
	} = $props();

	let password = $state('');
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
	<form
		class="card bg-surface-50-950 w-full max-w-sm space-y-4 p-6 shadow-xl"
		data-testid="wifi-connect-dialog"
		onsubmit={(e) => {
			e.preventDefault();
			onconnect(network.ssid, password);
		}}
	>
		<h3 class="h4">Connect to {network.ssid}</h3>
		{#if network.security}
			<label class="label">
				<span class="label-text">Password</span>
				<PasswordInput
					bind:value={password}
					placeholder="WiFi password"
					data-testid="wifi-dialog-password"
				/>
			</label>
		{/if}
		<div class="flex justify-end gap-2">
			<button type="button" class="btn preset-tonal-surface" onclick={oncancel}>Cancel</button>
			<button type="submit" class="btn preset-tonal-primary" data-testid="wifi-dialog-connect">
				Connect
			</button>
		</div>
	</form>
</div>
