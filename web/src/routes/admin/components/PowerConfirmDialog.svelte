<script lang="ts">
	import ConfirmDialog from '$lib/ConfirmDialog.svelte';
	import { requestPower, type PowerAction } from '$lib/power';

	let {
		action,
		onclose
	}: {
		action: PowerAction;
		onclose: () => void;
	} = $props();

	let busy = $state(false);
	let pollTimer: ReturnType<typeof setTimeout> | null = null;
	// A reboot keeps answering /healthz while systemd stops units, so reloading on
	// the first success would land on a dying server. Wait to see it go down first.
	let seenDown = false;

	const isReboot = $derived(action === 'reboot');
	const title = $derived(isReboot ? 'Reboot the frame?' : 'Shut the frame down?');
	const confirmLabel = $derived(isReboot ? 'Reboot' : 'Shut down');

	async function handleConfirm() {
		busy = true;
		const ok = await requestPower(action);
		if (!ok) {
			busy = false;
			return;
		}
		// Only a reboot comes back on its own.
		if (isReboot) pollHealthz();
	}

	function pollHealthz() {
		pollTimer = setTimeout(async () => {
			try {
				const res = await fetch('/healthz');
				if (res.ok && seenDown) {
					window.location.reload();
					return;
				}
				if (!res.ok) seenDown = true;
			} catch {
				seenDown = true;
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

<ConfirmDialog
	open
	{title}
	{confirmLabel}
	{busy}
	dialogTestid="power-dialog-{action}"
	confirmTestid="power-confirm-{action}"
	onconfirm={handleConfirm}
	{onclose}
>
	{#if !busy}
		{#if isReboot}
			The whole device restarts. Photos keep playing once it is back, usually within a minute.
		{:else}
			The device powers off and stays off. Someone has to physically power it on again, or cut and
			restore its power, before the frame comes back.
		{/if}
	{:else if isReboot}
		Rebooting. Waiting for the frame to come back online.
	{:else}
		Powering off. This page will stop responding shortly.
	{/if}
</ConfirmDialog>
