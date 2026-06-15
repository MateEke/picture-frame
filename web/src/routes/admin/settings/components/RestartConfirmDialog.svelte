<script lang="ts">
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
	import { restartFrame } from '$lib/config';

	let {
		oncancel
	}: {
		oncancel: () => void;
	} = $props();

	let restarting = $state(false);
	let pollTimer: ReturnType<typeof setTimeout> | null = null;

	async function handleConfirm() {
		restarting = true;
		const ok = await restartFrame();
		if (!ok) {
			restarting = false;
			return;
		}
		// Do not notify the parent here: calling onconfirm would set showRestartDialog=false,
		// scheduling component destruction, which cancels pollTimer before it fires.
		// The page reload from pollHealthz makes the parent notification redundant.
		pollHealthz();
	}

	function pollHealthz() {
		pollTimer = setTimeout(async () => {
			try {
				const res = await fetch('/healthz');
				if (res.ok) {
					window.location.reload();
					return;
				}
			} catch {
				// frame not yet back up, keep polling
			}
			pollHealthz();
		}, 2000);
	}

	$effect(() => {
		return () => {
			if (pollTimer !== null) clearTimeout(pollTimer);
		};
	});
</script>

<Dialog
	open
	onOpenChange={(d: { open: boolean }) => {
		if (!d.open && !restarting) oncancel();
	}}
>
	<Portal>
		<Dialog.Backdrop class="fixed inset-0 z-50 bg-black/50" />
		<Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
			{#if restarting}
				<Dialog.Content
					class="card bg-surface-100-900 w-full max-w-sm space-y-4 p-6 text-center shadow-xl"
				>
					<Dialog.Title class="text-lg font-semibold">Restarting…</Dialog.Title>
					<Dialog.Description class="text-surface-500-400 text-sm">
						Waiting for the frame to come back online.
					</Dialog.Description>
				</Dialog.Content>
			{:else}
				<Dialog.Content
					data-testid="restart-dialog"
					class="card bg-surface-100-900 w-full max-w-sm space-y-4 p-6 shadow-xl"
				>
					<Dialog.Title class="h4">Restart frame?</Dialog.Title>
					<Dialog.Description class="text-surface-600-300 text-sm">
						The frame will restart and be unreachable for a few seconds. Unsaved changes will be
						lost.
					</Dialog.Description>
					<div class="flex justify-end gap-2">
						<button
							type="button"
							class="btn preset-tonal-surface"
							data-testid="restart-cancel"
							onclick={oncancel}
						>
							Cancel
						</button>
						<button
							type="button"
							class="btn preset-tonal-error"
							data-testid="restart-confirm"
							onclick={handleConfirm}
						>
							Restart
						</button>
					</div>
				</Dialog.Content>
			{/if}
		</Dialog.Positioner>
	</Portal>
</Dialog>
