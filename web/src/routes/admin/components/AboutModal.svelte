<script lang="ts">
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
	import { ImageIcon, ChevronDownIcon, RefreshCwIcon } from '@lucide/svelte';
	import { loadLicenses, checkForUpdate } from '$lib/updater';

	let {
		open,
		current,
		platform,
		lastCheck,
		onclose
	}: {
		open: boolean;
		current: string;
		platform: string;
		lastCheck?: string;
		onclose: () => void;
	} = $props();

	let licenses = $state<string | null>(null);
	let licensesState = $state<'idle' | 'loading' | 'error'>('idle');
	let showLicenses = $state(false);
	let checking = $state(false);

	async function handleCheck() {
		checking = true;
		await checkForUpdate(lastCheck);
		checking = false;
	}

	async function toggleLicenses() {
		showLicenses = !showLicenses;
		if (!showLicenses || licenses !== null || licensesState === 'loading') return;
		licensesState = 'loading';
		const text = await loadLicenses(window.fetch);
		if (text === null) {
			licensesState = 'error';
		} else {
			licenses = text;
			licensesState = 'idle';
		}
	}
</script>

<Dialog
	{open}
	onOpenChange={(d: { open: boolean }) => {
		if (!d.open) onclose();
	}}
>
	<Portal>
		<Dialog.Backdrop class="fixed inset-0 z-50 bg-black/50" />
		<Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
			<Dialog.Content
				class="card bg-surface-100-900 flex max-h-[85vh] w-full max-w-lg flex-col gap-4 p-6"
				data-testid="about-modal"
			>
				<div class="flex items-center gap-3">
					<div
						class="bg-primary-500/15 text-primary-500 grid size-11 shrink-0 place-items-center rounded-xl"
					>
						<ImageIcon class="size-6" />
					</div>
					<div>
						<Dialog.Title class="text-lg font-semibold">Picture Frame</Dialog.Title>
						<p class="text-surface-500-400 text-sm">A frame for the photos you love.</p>
					</div>
				</div>

				<dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
					<div>
						<dt class="text-surface-500-400 text-xs">Version</dt>
						<dd class="font-medium tabular-nums" data-testid="about-version">{current}</dd>
					</div>
					<div>
						<dt class="text-surface-500-400 text-xs">Platform</dt>
						<dd class="font-medium" data-testid="about-platform">{platform}</dd>
					</div>
				</dl>

				<button
					type="button"
					class="btn btn-sm preset-tonal-surface flex w-fit items-center gap-1.5"
					onclick={handleCheck}
					disabled={checking}
					data-testid="about-check"
				>
					<RefreshCwIcon class="size-4 {checking ? 'animate-spin' : ''}" />
					{checking ? 'Checking…' : 'Check for updates'}
				</button>

				<div class="border-surface-300-700 border-t pt-3">
					<button
						type="button"
						class="text-surface-600-300 hover:text-primary-500 flex w-full items-center justify-between text-sm"
						onclick={toggleLicenses}
						data-testid="about-licenses-toggle"
					>
						<span>Third-party licenses</span>
						<ChevronDownIcon
							class="size-4 transition-transform {showLicenses ? 'rotate-180' : ''}"
						/>
					</button>
					{#if showLicenses}
						<div class="mt-2">
							{#if licensesState === 'loading'}
								<p class="text-surface-500-400 text-sm">Loading…</p>
							{:else if licensesState === 'error'}
								<p class="text-error-500 text-sm">Couldn't load license notices.</p>
							{:else}
								<pre
									class="bg-surface-200-800 max-h-64 overflow-auto rounded-lg p-3 text-xs leading-relaxed whitespace-pre-wrap"
									data-testid="about-licenses">{licenses}</pre>
							{/if}
						</div>
					{/if}
				</div>

				<div class="flex justify-end">
					<button class="btn preset-tonal-surface" onclick={onclose} data-testid="about-close">
						Close
					</button>
				</div>
			</Dialog.Content>
		</Dialog.Positioner>
	</Portal>
</Dialog>
