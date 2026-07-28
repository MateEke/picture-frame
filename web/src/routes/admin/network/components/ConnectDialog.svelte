<script lang="ts">
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
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

<Dialog
	open
	onOpenChange={(d: { open: boolean }) => {
		if (!d.open) oncancel();
	}}
>
	<Portal>
		<Dialog.Backdrop class="fixed inset-0 z-50 bg-black/50" />
		<Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
			<Dialog.Content
				class="card bg-surface-100-900 w-full max-w-sm space-y-4 p-6"
				data-testid="wifi-connect-dialog"
			>
				<Dialog.Title class="h4">Connect to {network.ssid}</Dialog.Title>
				<form
					class="space-y-4"
					onsubmit={(e) => {
						e.preventDefault();
						onconnect(network.ssid, password);
					}}
				>
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
						<button type="button" class="btn preset-tonal-surface" onclick={oncancel}>Cancel</button
						>
						<button
							type="submit"
							class="btn preset-tonal-primary"
							data-testid="wifi-dialog-connect"
						>
							Connect
						</button>
					</div>
				</form>
			</Dialog.Content>
		</Dialog.Positioner>
	</Portal>
</Dialog>
