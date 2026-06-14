<script lang="ts">
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

<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
	{#if restarting}
		<div class="card bg-surface-50-950 w-full max-w-sm space-y-4 p-6 text-center shadow-xl">
			<p class="text-lg font-semibold">Restarting…</p>
			<p class="text-surface-500-400 text-sm">Waiting for the frame to come back online.</p>
		</div>
	{:else}
		<div
			data-testid="restart-dialog"
			class="card bg-surface-50-950 w-full max-w-sm space-y-4 p-6 shadow-xl"
		>
			<h3 class="h4">Restart frame?</h3>
			<p class="text-surface-600-300 text-sm">
				The frame will restart and be unreachable for a few seconds. Unsaved changes will be lost.
			</p>
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
		</div>
	{/if}
</div>
