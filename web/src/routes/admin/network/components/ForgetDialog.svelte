<script lang="ts">
	import ConfirmDialog from '$lib/ConfirmDialog.svelte';

	let {
		target,
		outcome,
		onconfirm,
		oncancel
	}: {
		target: { ssid: string; active: boolean };
		outcome: string;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();
</script>

<ConfirmDialog
	open
	title="Forget {target.ssid}?"
	confirmLabel="Forget"
	dialogTestid="wifi-forget-dialog"
	confirmTestid="wifi-forget-confirm"
	{onconfirm}
	onclose={oncancel}
>
	{#if target.active}
		<span class="text-warning-700-300">
			This is the network the frame is currently using. Forgetting it disconnects the frame.
			{outcome}
		</span>
	{:else}
		<span class="opacity-80">The saved password for this network will be removed.</span>
	{/if}
</ConfirmDialog>
