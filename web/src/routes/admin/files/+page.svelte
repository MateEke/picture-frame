<script lang="ts">
	import type { PageProps } from './$types';
	import { invalidate } from '$app/navigation';
	import { onDestroy } from 'svelte';
	import { DownloadIcon, FileIcon, FolderOpenIcon, Trash2Icon } from '@lucide/svelte';
	import { FileUpload } from '@skeletonlabs/skeleton-svelte';
	import { CloudUploadIcon } from '@lucide/svelte';
	import ConfirmDialog from '$lib/ConfirmDialog.svelte';
	import { deleteFile, uploadFiles } from '$lib/files';
	import { formatBytes, unitsEn } from '$lib/touch';
	import { toaster } from '$lib/toaster';
	import { getSSEContext } from '$lib/sse.svelte';
	import { fileUploadMessage } from '../images/uploadFeedback';

	let { data }: PageProps = $props();
	const sse = getSSEContext();

	let progress = $state<{ sent: number; total: number } | null>(null);
	let abort: AbortController | null = null;
	let pendingDelete = $state<string | null>(null);
	let deleting = $state(false);

	// Uploads from the frame's own screen or another phone show up live.
	let lastRev = -1;
	$effect(() => {
		const rev = sse.libraryRev;
		if (lastRev !== -1 && rev !== lastRev) invalidate('app:files');
		lastRev = rev;
	});

	onDestroy(() => abort?.abort());

	const dateFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });

	async function upload(details: { files: File[] }) {
		if (details.files.length === 0) return;
		abort = new AbortController();
		progress = { sent: 0, total: 1 };
		const result = await uploadFiles(details.files, {
			signal: abort.signal,
			onProgress: (sent, total) => (progress = { sent, total })
		});
		abort = null;
		progress = null;
		const { type, ...message } = fileUploadMessage(result);
		toaster[type](message);
		await invalidate('app:files');
	}

	async function confirmDelete() {
		if (!pendingDelete) return;
		deleting = true;
		try {
			if (!(await deleteFile(pendingDelete))) {
				toaster.error({ title: 'Could not delete file', description: 'Server returned an error' });
			}
			await invalidate('app:files');
		} finally {
			deleting = false;
			pendingDelete = null;
		}
	}

	function href(name: string, download = false): string {
		return `/files/${encodeURIComponent(name)}${download ? '?download=true' : ''}`;
	}
</script>

<div class="mx-auto w-full max-w-5xl space-y-6">
	<header class="space-y-1">
		<h1 class="h2">Files</h1>
		<p class="text-surface-500-400 text-sm">
			Videos, documents and anything else that isn't a slideshow photo. They can be opened on the
			frame's touch screen.
			{#if data.freeBytes !== null}
				<span class="whitespace-nowrap">{formatBytes(data.freeBytes, unitsEn)} free.</span>
			{/if}
		</p>
	</header>

	{#if progress}
		<div
			class="border-surface-300-700 space-y-2 rounded-lg border-2 p-6 text-center"
			data-testid="files-upload-progress"
		>
			<p class="font-medium">
				Uploading… {Math.round((progress.sent / Math.max(1, progress.total)) * 100)}%
			</p>
			<div class="bg-surface-300-700 mx-auto h-1.5 max-w-xs overflow-hidden rounded-full">
				<div
					class="bg-primary-500 h-full"
					style="width: {(progress.sent / Math.max(1, progress.total)) * 100}%"
				></div>
			</div>
			<button
				type="button"
				class="btn btn-sm preset-outlined-surface-500"
				onclick={() => abort?.abort()}
			>
				Stop
			</button>
		</div>
	{:else}
		<FileUpload maxFiles={200} onFileAccept={upload}>
			<FileUpload.Dropzone
				class="border-surface-300-700 hover:border-primary-500 cursor-pointer gap-2 rounded-lg border-2 border-dashed p-6 text-center"
			>
				<FileUpload.HiddenInput data-testid="files-upload-input" />
				<CloudUploadIcon class="text-primary-500 size-6" />
				<p class="font-medium">Drop files here, or click to choose</p>
			</FileUpload.Dropzone>
		</FileUpload>
	{/if}

	{#if data.files === null}
		<p class="text-error-500">Could not load files: {data.error}</p>
	{:else if data.files.length === 0}
		<div
			class="card bg-surface-100-900 text-surface-500-400 flex flex-col items-center gap-2 p-10 text-center"
		>
			<FolderOpenIcon class="size-8" />
			<p class="font-medium">No files yet</p>
		</div>
	{:else}
		<!-- /files/* is a server route, not a SvelteKit page, so there's nothing to resolve(). -->
		<!-- eslint-disable svelte/no-navigation-without-resolve -->
		<ul class="card bg-surface-100-900 divide-surface-200-800 divide-y" data-testid="files-list">
			{#each data.files as f (f.name)}
				<li class="flex items-center gap-3 p-3" data-testid="file-row-{f.name}">
					<FileIcon class="text-primary-500 size-6 shrink-0" />
					<a
						class="anchor min-w-0 flex-1 truncate"
						href={href(f.name)}
						target="_blank"
						rel="noopener"
					>
						{f.original}
					</a>
					<span class="text-surface-500-400 hidden text-sm sm:inline">
						{formatBytes(f.size, unitsEn)} · {dateFmt.format(new Date(f.added))}
					</span>
					<a
						class="btn-icon preset-tonal"
						href={href(f.name, true)}
						aria-label="Download {f.original}"
					>
						<DownloadIcon class="size-4" />
					</a>
					<button
						type="button"
						class="btn-icon preset-tonal-error"
						aria-label="Delete {f.original}"
						data-testid="file-delete-{f.name}"
						onclick={() => (pendingDelete = f.name)}
					>
						<Trash2Icon class="size-4" />
					</button>
				</li>
			{/each}
		</ul>
		<!-- eslint-enable svelte/no-navigation-without-resolve -->
	{/if}
</div>

<ConfirmDialog
	open={pendingDelete !== null}
	title="Delete file?"
	confirmLabel={deleting ? 'Deleting…' : 'Delete'}
	busy={deleting}
	confirmTestid="file-delete-confirm"
	onconfirm={confirmDelete}
	onclose={() => (pendingDelete = null)}
>
	This will permanently delete <span class="font-mono">{pendingDelete}</span>.
</ConfirmDialog>
