<script lang="ts">
	import type { PageProps } from './$types';
	import { SvelteSet } from 'svelte/reactivity';
	import { invalidate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import { flip } from 'svelte/animate';
	import {
		ImageIcon,
		CheckIcon,
		MonitorPlayIcon,
		Trash2Icon,
		ArrowUpIcon,
		ArrowDownIcon,
		ChevronsUpIcon,
		ChevronsDownIcon
	} from '@lucide/svelte';
	import { dndzone, SHADOW_ITEM_MARKER_PROPERTY_NAME, type DndEvent } from 'svelte-dnd-action';
	import { deleteImage, deleteImages, setImageOrder } from '$lib/images';
	import { moveUp, moveDown, moveToStart, moveToEnd } from '$lib/reorder';
	import { syncLibrary } from '$lib/library';
	import { getSSEContext } from '$lib/sse.svelte';
	import ConfirmDialog from '$lib/ConfirmDialog.svelte';
	import Cropper from './components/Cropper.svelte';
	import UploadDropzone from './components/UploadDropzone.svelte';
	import ImmichStatus from './components/ImmichStatus.svelte';
	import Lightbox from './components/Lightbox.svelte';

	let { data }: PageProps = $props();
	const sse = getSSEContext();

	let currentFile = $state<File | null>(null);
	let pendingDelete = $state<string | null>(null);
	let deleting = $state(false);
	let brokenImages = new SvelteSet<string>();
	let lightboxName = $state<string | null>(null);
	let selecting = $state(false);
	let selected = new SvelteSet<string>();
	let bulkConfirm = $state(false);
	let syncing = $state(false);

	const isFs = $derived(data.library?.backend === 'fs');
	const shuffleOn = $derived(data.shuffleOn ?? false);
	// On-screen photos to badge (a split slide shows two).
	const onScreen = $derived(new Set(sse.image?.names ?? []));
	const images = $derived(data.images ?? []);

	let arranging = $state(false);

	type DndItem = { id: string; [key: string]: unknown };

	let arrangeItems = $state<DndItem[] | null>(null);
	let savedOrder: string[] = []; // last server-confirmed order; revert target on a failed save
	let changed = false; // a reorder happened this arrange session
	let saveTimer: ReturnType<typeof setTimeout> | undefined;
	let inFlightSave: Promise<unknown> = Promise.resolve(); // so Done's commit lands after a pending save

	const orderedNames = $derived(
		arrangeItems ? arrangeItems.map((i) => i.id) : (data.images ?? []).map((i) => i.name)
	);

	function isShadowItem(item: DndItem): boolean {
		return SHADOW_ITEM_MARKER_PROPERTY_NAME in item && !!item[SHADOW_ITEM_MARKER_PROPERTY_NAME];
	}

	function handleDnd(e: CustomEvent<DndEvent<DndItem>>) {
		arrangeItems = e.detail.items;
		if (e.type === 'finalize') {
			const ids = e.detail.items.filter((i) => !isShadowItem(i)).map((i) => i.id);
			applyOrder(ids);
		}
	}

	function startArranging() {
		arranging = true;
		changed = false;
		arrangeItems = (data.images ?? [])
			.filter((i) => !brokenImages.has(i.name))
			.map((i) => ({ id: i.name }));
		savedOrder = arrangeItems.map((i) => i.id);
	}

	async function stopArranging() {
		arranging = false;
		clearTimeout(saveTimer);
		saveTimer = undefined;
		if (changed && arrangeItems) {
			await inFlightSave; // let a fired debounce land first so commit wins
			// commit:true tells the server to restart the cycle with the new order.
			await setImageOrder(
				arrangeItems.map((i) => i.id),
				true
			);
		}
		arrangeItems = null;
		changed = false;
		await invalidate('/api/images');
	}

	function applyOrder(next: string[]) {
		changed = true;
		arrangeItems = next.map((id) => ({ id }));
		clearTimeout(saveTimer);
		saveTimer = setTimeout(() => {
			inFlightSave = setImageOrder(next).then((ok) => {
				if (ok) savedOrder = next;
				else if (arranging) arrangeItems = savedOrder.map((id) => ({ id }));
			});
		}, 400);
	}

	function move(name: string, fn: (items: readonly string[], index: number) => string[]) {
		if (!arrangeItems) return;
		const current = arrangeItems.map((i) => i.id);
		const index = current.indexOf(name);
		if (index < 0) return;
		const next = fn(current, index);
		if (next.length === current.length && next.every((n, i) => n === current[i])) return;
		applyOrder(next);
	}

	function subtitle(): string {
		if (data.library === null) return 'Manage the photos shown on your frame.';
		if (data.library.backend === 'immich') return 'Your frame mirrors an Immich album.';
		return 'Add photos and manage what plays on your frame.';
	}

	function openImage(name: string) {
		if (arranging) return;
		if (selecting) {
			toggleSelect(name);
			return;
		}
		lightboxName = name;
	}

	function toggleSelect(name: string) {
		if (selected.has(name)) selected.delete(name);
		else selected.add(name);
	}

	function startSelecting() {
		selecting = true;
		selected.clear();
	}

	function cancelSelecting() {
		selecting = false;
		selected.clear();
	}

	function requestDelete(name: string) {
		lightboxName = null;
		pendingDelete = name;
	}

	async function confirmDelete() {
		if (!pendingDelete) return;
		deleting = true;
		try {
			await deleteImage(pendingDelete);
		} finally {
			deleting = false;
			pendingDelete = null;
		}
	}

	async function confirmBulkDelete() {
		deleting = true;
		try {
			await deleteImages([...selected]);
		} finally {
			deleting = false;
			bulkConfirm = false;
			selected.clear();
			selecting = false;
		}
	}

	async function handleSync() {
		syncing = true;
		try {
			await syncLibrary(data.library?.sync?.last_sync);
		} finally {
			syncing = false;
		}
	}

	// Refresh Immich sync status periodically; pause while the tab is hidden.
	onMount(() => {
		if (data.library?.backend !== 'immich') return;
		const id = setInterval(() => {
			if (document.visibilityState !== 'visible') return;
			invalidate('/api/library');
			invalidate('/api/images');
		}, 30_000);
		return () => clearInterval(id);
	});
</script>

{#snippet photoCard(name: string)}
	<div
		data-testid="photo-card-{name}"
		class="group bg-surface-200-800 relative aspect-video overflow-hidden rounded-lg {selected.has(
			name
		)
			? 'ring-primary-500 ring-2'
			: ''} {onScreen.has(name) ? 'ring-success-500 ring-2' : ''}"
	>
		<button
			type="button"
			class="absolute inset-0 size-full cursor-pointer"
			aria-label={selecting ? `Select ${name}` : `Preview ${name}`}
			onclick={() => openImage(name)}
		>
			<img
				src="/img/{name}"
				alt={name}
				data-testid="photo-thumb"
				class="size-full object-cover transition-transform group-hover:scale-105"
				loading="lazy"
				decoding="async"
				onerror={() => brokenImages.add(name)}
			/>
		</button>

		{#if onScreen.has(name)}
			<span
				data-testid="photo-onscreen"
				class="preset-filled-success-500 pointer-events-none absolute top-1.5 left-1.5 flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium shadow"
			>
				<MonitorPlayIcon class="size-3" />On screen
			</span>
		{/if}

		{#if selecting}
			<span
				class="pointer-events-none absolute top-1.5 right-1.5 grid size-6 place-items-center rounded-full border-2 {selected.has(
					name
				)
					? 'border-primary-500 bg-primary-500 text-white'
					: 'border-white/80 bg-black/30'}"
			>
				{#if selected.has(name)}<CheckIcon class="size-4" />{/if}
			</span>
		{:else if isFs && !arranging}
			<button
				type="button"
				data-testid="photo-delete-{name}"
				class="btn preset-filled-error-500 absolute top-1.5 right-1.5 hidden px-2 py-1 text-xs group-hover:flex"
				onclick={() => requestDelete(name)}
			>
				<Trash2Icon class="size-3.5" />
			</button>
		{/if}

		{#if arranging}
			<div class="absolute inset-x-1 bottom-1 flex justify-center gap-1 rounded bg-black/40 p-1">
				<button
					type="button"
					class="btn-icon btn-icon-sm preset-tonal"
					data-testid="photo-move-start-{name}"
					aria-label="Move {name} to start"
					onclick={() => move(name, moveToStart)}><ChevronsUpIcon class="size-4" /></button
				>
				<button
					type="button"
					class="btn-icon btn-icon-sm preset-tonal"
					data-testid="photo-move-up-{name}"
					aria-label="Move {name} up"
					onclick={() => move(name, moveUp)}><ArrowUpIcon class="size-4" /></button
				>
				<button
					type="button"
					class="btn-icon btn-icon-sm preset-tonal"
					data-testid="photo-move-down-{name}"
					aria-label="Move {name} down"
					onclick={() => move(name, moveDown)}><ArrowDownIcon class="size-4" /></button
				>
				<button
					type="button"
					class="btn-icon btn-icon-sm preset-tonal"
					data-testid="photo-move-end-{name}"
					aria-label="Move {name} to end"
					onclick={() => move(name, moveToEnd)}><ChevronsDownIcon class="size-4" /></button
				>
			</div>
		{/if}
	</div>
{/snippet}

<div class="mx-auto w-full max-w-5xl space-y-6">
	<header class="space-y-1">
		<h1 class="h2">Photos</h1>
		<p class="text-surface-500-400 text-sm">{subtitle()}</p>
	</header>

	{#if data.library === null}
		<div class="card bg-surface-100-900 reveal space-y-2 p-6">
			<h2 class="h4 text-error-500">Library status unavailable</h2>
			<p class="text-surface-500-400 text-sm">
				{data.libraryError ?? 'Could not reach /api/library.'}
				Uploads and deletes are disabled until status can be confirmed.
			</p>
		</div>
	{:else if isFs}
		{#if currentFile}
			<div class="card bg-surface-100-900 reveal p-4">
				<Cropper
					file={currentFile}
					onUploaded={() => (currentFile = null)}
					onCancel={() => (currentFile = null)}
				/>
			</div>
		{:else}
			<div class="reveal"><UploadDropzone onFile={(f) => (currentFile = f)} /></div>
		{/if}
	{:else}
		<ImmichStatus sync={data.library.sync} shareUrl={data.shareUrl} {syncing} onSync={handleSync} />
	{/if}

	{#if data.images === null}
		<p class="reveal text-error-500 delay-75">Could not load images: {data.imagesError}</p>
	{:else if images.length > 0}
		<div class="space-y-4">
			<div class="reveal flex items-center justify-between gap-2 delay-75">
				<div class="flex items-center gap-2">
					<ImageIcon class="text-primary-500 size-5" />
					<h2 class="h4">Your photos</h2>
					<span class="text-surface-500-400 text-sm">({images.length})</span>
				</div>
				{#if isFs}
					{#if arranging}
						<button
							class="btn btn-sm preset-tonal-surface"
							data-testid="photos-arrange-done"
							onclick={stopArranging}>Done</button
						>
					{:else if selecting}
						<div class="flex items-center gap-2">
							<button
								class="btn btn-sm preset-filled-error-500 flex items-center gap-1.5"
								data-testid="photos-bulk-delete"
								onclick={() => (bulkConfirm = true)}
								disabled={selected.size === 0}
							>
								<Trash2Icon class="size-4" />Delete {selected.size}
							</button>
							<button
								class="btn btn-sm preset-tonal-surface"
								data-testid="photos-select-cancel"
								onclick={cancelSelecting}
							>
								Cancel
							</button>
						</div>
					{:else}
						<div class="flex items-center gap-2">
							<button
								class="btn btn-sm preset-tonal-surface"
								data-testid="photos-arrange"
								onclick={startArranging}>Arrange</button
							>
							<button
								class="btn btn-sm preset-tonal-surface"
								data-testid="photos-select"
								onclick={startSelecting}>Select</button
							>
						</div>
					{/if}
				{/if}
			</div>

			{#if arranging && shuffleOn}
				<div class="card preset-tonal-warning p-3 text-sm" data-testid="photos-shuffle-hint">
					Shuffle is on, so this order will not affect playback until you turn it off in
					<a class="anchor" href={resolve('/admin/settings')}>Settings</a>.
				</div>
			{/if}

			{#if arranging && arrangeItems}
				<div
					class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4"
					use:dndzone={{ items: arrangeItems, flipDurationMs: 150 }}
					onconsider={handleDnd}
					onfinalize={handleDnd}
				>
					{#each arrangeItems as item (item.id)}
						<div animate:flip={{ duration: 150 }}>
							{#if isShadowItem(item)}
								<div class="bg-surface-200-800 aspect-video rounded-lg opacity-50"></div>
							{:else}
								{@render photoCard(item.id)}
							{/if}
						</div>
					{/each}
				</div>
			{:else}
				<div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4">
					{#each orderedNames as name (name)}
						{#if !brokenImages.has(name)}
							{@render photoCard(name)}
						{/if}
					{/each}
				</div>
			{/if}
		</div>
	{:else if !isFs}
		<div
			class="card bg-surface-100-900 text-surface-500-400 reveal flex flex-col items-center gap-2 p-10 text-center delay-75"
		>
			<ImageIcon class="size-8" />
			<p class="font-medium">No photos yet</p>
			<p class="text-sm">Add photos to your Immich album and they’ll appear here.</p>
		</div>
	{/if}
</div>

<Lightbox
	name={lightboxName}
	canDelete={isFs}
	onClose={() => (lightboxName = null)}
	onDelete={requestDelete}
/>

<ConfirmDialog
	open={pendingDelete !== null}
	title="Delete photo?"
	confirmLabel={deleting ? 'Deleting…' : 'Delete'}
	busy={deleting}
	confirmTestid="photo-delete-confirm"
	onconfirm={confirmDelete}
	onclose={() => (pendingDelete = null)}
>
	This will permanently delete <span class="font-mono">{pendingDelete}</span>.
</ConfirmDialog>

<ConfirmDialog
	open={bulkConfirm}
	title="Delete {selected.size} photos?"
	confirmLabel={deleting ? 'Deleting…' : `Delete ${selected.size}`}
	busy={deleting}
	confirmTestid="photos-bulk-confirm"
	onconfirm={confirmBulkDelete}
	onclose={() => (bulkConfirm = false)}
>
	This will permanently delete the selected photos.
</ConfirmDialog>
