<script lang="ts">
	import { onMount } from 'svelte';
	import { DownloadIcon, RotateCcwIcon, ExternalLinkIcon } from '@lucide/svelte';
	import ConfirmDialog from '$lib/ConfirmDialog.svelte';
	import {
		applyUpdate,
		followUpdate,
		isApplyInProgress,
		type UpdateStatusResponse,
		type UpdatePhase
	} from '$lib/updater';

	let { status }: { status: UpdateStatusResponse | null } = $props();

	let phase = $state<UpdatePhase>('idle');
	let confirmOpen = $state(false);
	const applying = $derived(phase !== 'idle');

	// Adopt an apply already running when this panel loads (e.g. tab reloaded mid-update): keeps
	// the button disabled against a duplicate trigger and follows it to completion.
	onMount(() => {
		if (!status || !isApplyInProgress(status.phase)) return;
		phase = status.phase;
		followUpdate(status.last_result_seq ?? 0, (p) => (phase = p)).finally(() => {
			phase = 'idle';
		});
	});

	const available = $derived(status?.available ?? false);
	const lastFailed = $derived(
		status?.last_result?.startsWith('failed') ||
			status?.last_result?.startsWith('rolled back') ||
			false
	);
	// Standalone notice for a rollback when no newer update is on offer to retry.
	const rolledBack = $derived(
		!available && (status?.last_result?.startsWith('rolled back') ?? false)
	);

	const phaseLabel: Record<UpdatePhase, string> = {
		idle: 'Updating…',
		checking: 'Checking…',
		downloading: 'Downloading…',
		verifying: 'Verifying…',
		applying: 'Applying…'
	};

	async function handleApply() {
		confirmOpen = false;
		phase = 'downloading';
		await applyUpdate((p) => (phase = p));
		phase = 'idle';
	}
</script>

{#if available}
	<div
		class="card border-primary-500/30 bg-primary-500/5 reveal flex flex-col gap-4 border p-5 sm:flex-row sm:items-center"
		data-testid="update-panel"
	>
		<div
			class="bg-primary-500/15 text-primary-500 grid size-11 shrink-0 place-items-center rounded-xl"
		>
			<DownloadIcon class="size-6 {applying ? 'animate-pulse' : ''}" />
		</div>
		<div class="min-w-0 flex-1">
			<p class="font-semibold" data-testid="update-panel-headline">Update available</p>
			<p class="text-surface-600-300 text-sm">
				Version <span class="font-medium">{status?.latest}</span> is ready to install.
				{#if status?.notes_url}
					<a
						href={status.notes_url}
						target="_blank"
						rel="external noreferrer"
						class="text-primary-500 ml-1 inline-flex items-center gap-1 hover:underline"
						data-testid="update-notes"
					>
						Release notes<ExternalLinkIcon class="size-3.5" />
					</a>
				{/if}
			</p>
			{#if lastFailed && !applying}
				<p class="text-warning-600-400 mt-1 text-xs" data-testid="update-retry-note">
					The last attempt didn't complete. You can try again.
				</p>
			{/if}
		</div>
		<button
			class="btn preset-filled-primary-500 shrink-0 self-start sm:self-auto"
			data-testid="update-now"
			onclick={() => (confirmOpen = true)}
			disabled={applying}
		>
			{applying ? phaseLabel[phase] : 'Update now'}
		</button>
	</div>
{:else if rolledBack}
	<div
		class="card border-warning-500/30 bg-warning-500/5 reveal flex items-center gap-4 border p-5"
		data-testid="update-rolled-back"
	>
		<div
			class="bg-warning-500/15 text-warning-600-400 grid size-11 shrink-0 place-items-center rounded-xl"
		>
			<RotateCcwIcon class="size-6" />
		</div>
		<div class="min-w-0">
			<p class="font-semibold">Last update was rolled back</p>
			<p class="text-surface-600-300 text-sm">
				A newer build didn't start, so the frame safely returned to <span class="font-medium"
					>{status?.current}</span
				>.
			</p>
		</div>
	</div>
{/if}

<ConfirmDialog
	open={confirmOpen}
	title="Install {status?.latest}?"
	confirmLabel="Update now"
	confirmClass="preset-filled-primary-500"
	busy={applying}
	dialogTestid="update-confirm"
	confirmTestid="update-confirm-go"
	onconfirm={handleApply}
	onclose={() => (confirmOpen = false)}
>
	The frame will download and verify the update, then restart into the new version. It will be
	unreachable for a few seconds. If the new version doesn't start, it rolls back automatically.
</ConfirmDialog>
