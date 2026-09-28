<script lang="ts">
	import {
		FileAudioIcon,
		FileIcon,
		FileImageIcon,
		FileTextIcon,
		FileVideoIcon,
		Trash2Icon,
		XIcon
	} from '@lucide/svelte';
	import { onDestroy, onMount } from 'svelte';
	import { getSSEContext } from '$lib/sse.svelte';
	import {
		fetchDeviceInfo,
		fetchFiles,
		fileKind,
		formatBytes,
		removeFile,
		type DeviceInfo,
		type FileItem,
		type FileKind
	} from '$lib/touch';
	import { fi } from './fi';
	import ConfirmSheet from './ConfirmSheet.svelte';
	import Notice from './Notice.svelte';
	import QrCode from './QrCode.svelte';

	const sse = getSSEContext();

	let files = $state<FileItem[] | null>(null);
	let freeBytes = $state<number | null>(null);
	let open = $state<FileItem | null>(null);
	let confirm = $state<FileItem | null>(null);
	let text = $state<string | null>(null);
	let textTruncated = $state(false);
	let info = $state<DeviceInfo | null>(null);
	let error = $state<string | null>(null);
	let errorTimer: ReturnType<typeof setTimeout> | undefined;
	onDestroy(() => clearTimeout(errorTimer));

	$effect(() => {
		void sse.libraryRev;
		fetchFiles().then((l) => {
			if (!l) return;
			files = l.files;
			freeBytes = l.freeBytes;
		});
	});

	onMount(() => {
		fetchDeviceInfo().then((d) => (info = d));
	});

	const TEXT_PREVIEW_BYTES = 200_000;

	// Text previews read only the head of the file (Range) so a huge log can't
	// stall the renderer.
	$effect(() => {
		const f = open;
		text = null;
		textTruncated = false;
		if (!f || fileKind(f.mime) !== 'text') return;
		const ctl = new AbortController();
		fetch(`/files/${encodeURIComponent(f.name)}`, {
			headers: { Range: `bytes=0-${TEXT_PREVIEW_BYTES - 1}` },
			signal: ctl.signal
		})
			.then((r) => r.text())
			.then((t) => {
				text = t;
				textTruncated = f.size > TEXT_PREVIEW_BYTES;
			})
			.catch(() => undefined);
		return () => ctl.abort();
	});

	const icons: Record<FileKind, typeof FileIcon> = {
		image: FileImageIcon,
		video: FileVideoIcon,
		audio: FileAudioIcon,
		pdf: FileTextIcon,
		text: FileTextIcon,
		other: FileIcon
	};

	const dateFmt = new Intl.DateTimeFormat('fi-FI', { dateStyle: 'medium', timeStyle: 'short' });

	function src(f: FileItem): string {
		return `/files/${encodeURIComponent(f.name)}`;
	}

	function phoneUrl(f: FileItem): string {
		if (!info?.ip) return '';
		const port = location.port && location.port !== '80' ? `:${location.port}` : '';
		return `http://${info.ip}${port}${src(f)}?download=true`;
	}

	async function doDelete(f: FileItem) {
		confirm = null;
		if (await removeFile(f.name)) {
			files = (files ?? []).filter((x) => x.name !== f.name);
			if (open?.name === f.name) open = null;
		} else {
			error = fi.files.deleteFailed;
			clearTimeout(errorTimer);
			errorTimer = setTimeout(() => (error = null), 3000);
		}
	}
</script>

<div class="flex h-full flex-col" data-testid="touch-files">
	<header class="flex shrink-0 items-center justify-between border-b border-neutral-800 px-6 py-4">
		<h1 class="text-3xl font-semibold">{fi.tabs.files}</h1>
		{#if freeBytes !== null}
			<span class="text-lg text-neutral-400">{fi.files.free(formatBytes(freeBytes))}</span>
		{/if}
	</header>

	<div class="min-h-0 flex-1 overflow-y-auto overscroll-contain">
		{#if files && files.length === 0}
			<p class="p-8 text-2xl leading-relaxed text-neutral-400">{fi.files.empty}</p>
		{:else if files}
			<ul>
				{#each files as f (f.name)}
					{@const Icon = icons[fileKind(f.mime)]}
					<li class="border-b border-neutral-900">
						<button
							type="button"
							class="flex w-full items-center gap-5 px-6 py-5 text-left active:bg-neutral-900"
							data-testid="touch-file-item"
							data-name={f.name}
							onclick={() => (open = f)}
						>
							<Icon class="size-12 shrink-0 text-amber-300" strokeWidth={1.5} />
							<span class="min-w-0 flex-1">
								<span class="block truncate text-2xl">{f.original}</span>
								<span class="block text-lg text-neutral-400"
									>{formatBytes(f.size)} · {dateFmt.format(new Date(f.added))}</span
								>
							</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</div>

	{#if open}
		{@const kind = fileKind(open.mime)}
		<div
			class="fixed inset-0 z-30 flex flex-col bg-black"
			data-testid="touch-file-preview"
			role="dialog"
			aria-modal="true"
		>
			<header class="flex shrink-0 items-center gap-4 bg-neutral-950 p-4">
				<span class="min-w-0 flex-1 truncate text-2xl">{open.original}</span>
				<button
					type="button"
					class="flex size-18 items-center justify-center rounded-full bg-neutral-800"
					aria-label={fi.files.close}
					data-testid="touch-file-close"
					onclick={() => (open = null)}><XIcon class="size-10" /></button
				>
			</header>
			<div class="relative flex min-h-0 flex-1 items-center justify-center overflow-auto">
				{#if kind === 'image'}
					<img src={src(open)} alt="" class="max-h-full max-w-full object-contain" />
				{:else if kind === 'video'}
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						src={src(open)}
						controls
						autoplay
						playsinline
						preload="metadata"
						class="max-h-full max-w-full"
					></video>
				{:else if kind === 'audio'}
					<audio src={src(open)} controls autoplay class="w-4/5"></audio>
				{:else if kind === 'text'}
					<pre
						class="h-full w-full overflow-auto p-6 text-lg leading-relaxed whitespace-pre-wrap text-neutral-200">{text ??
							fi.files.loading}{textTruncated ? `\n\n${fi.files.textTruncated}` : ''}</pre>
				{:else}
					<div class="flex flex-col items-center gap-6 p-8 text-center">
						<p class="text-2xl text-neutral-300">{fi.files.noPreview}</p>
						{#if phoneUrl(open)}
							<p class="text-xl text-neutral-400">{fi.files.openOnPhone}</p>
							<QrCode text={phoneUrl(open)} size={300} />
						{/if}
					</div>
				{/if}
			</div>
			<footer class="shrink-0 bg-neutral-950 p-3">
				<button
					type="button"
					class="flex h-20 w-full items-center justify-center gap-3 rounded-2xl bg-red-900 text-xl active:bg-red-800"
					data-testid="touch-file-delete"
					onclick={() => (confirm = open)}
				>
					<Trash2Icon class="size-8" />{fi.files.delete}
				</button>
			</footer>
		</div>
	{/if}

	{#if confirm}
		<ConfirmSheet
			message={fi.files.confirmDelete(confirm.original)}
			confirmLabel={fi.files.delete}
			onConfirm={() => confirm && doDelete(confirm)}
			onCancel={() => (confirm = null)}
		/>
	{/if}

	{#if error}
		<Notice text={error} tone="error" />
	{/if}
</div>
