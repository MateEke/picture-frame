<script lang="ts">
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
	import PasswordInput from '$lib/PasswordInput.svelte';

	let {
		onconnect,
		oncancel
	}: {
		onconnect: (ssid: string, password: string) => void;
		oncancel: () => void;
	} = $props();

	let ssid = $state('');
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
				data-testid="wifi-hidden-dialog"
			>
				<Dialog.Title class="h4">Join a hidden network</Dialog.Title>
				<form
					class="space-y-4"
					onsubmit={(e) => {
						e.preventDefault();
						onconnect(ssid.trim(), password);
					}}
				>
					<label class="label">
						<span class="label-text">Network name (SSID)</span>
						<input
							class="input"
							bind:value={ssid}
							placeholder="Exact network name"
							data-testid="wifi-hidden-ssid"
							required
						/>
					</label>
					<label class="label">
						<span class="label-text">Password</span>
						<PasswordInput
							bind:value={password}
							placeholder="Leave empty for an open network"
							data-testid="wifi-hidden-password"
						/>
					</label>
					<div class="flex justify-end gap-2">
						<button type="button" class="btn preset-tonal-surface" onclick={oncancel}>Cancel</button
						>
						<button
							type="submit"
							class="btn preset-tonal-primary"
							data-testid="wifi-hidden-connect"
							disabled={ssid.trim() === ''}
						>
							Join
						</button>
					</div>
				</form>
			</Dialog.Content>
		</Dialog.Positioner>
	</Portal>
</Dialog>
