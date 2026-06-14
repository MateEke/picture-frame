<script lang="ts">
	import { Cropt } from 'cropt';
	import 'cropt/src/cropt.css';

	import type { Attachment } from 'svelte/attachments';

	import { uploadImage } from '$lib/images';
	import { toaster } from '$lib/toaster';

	interface Props {
		file: File;
		onUploaded: () => void;
		onCancel: () => void;
		class?: string;
	}

	let { file, onUploaded, onCancel, class: className = '' }: Props = $props();

	let cropt: Cropt | null = null;
	let boundFile: File | null = null;
	let uploading = $state(false);

	// Tracks the rendered boundary size so the CSS custom property stays in sync.
	let cropW = $state(0);
	let cropH = $derived(Math.floor((cropW * 9) / 16));

	const cropperAttachment: Attachment = (element) => {
		if (!(element instanceof HTMLElement)) return;

		let debounce: ReturnType<typeof setTimeout>;
		let first = true;

		const rebuild = (w: number) => {
			cropW = w;
			const viewport = { width: w, height: Math.floor((w * 9) / 16) };
			cropt?.destroy();
			boundFile = null;
			cropt = new Cropt(element, { viewport, mouseWheelZoom: 'ctrl' });
			bindFile(file);
		};

		let lastW = 0;
		const ro = new ResizeObserver(([entry]) => {
			const w = Math.floor(entry.contentRect.width);
			if (w === 0 || w === lastW) return;
			lastW = w;
			if (first) {
				first = false;
				rebuild(w); // no debounce on first observation, element just became visible
			} else {
				clearTimeout(debounce);
				debounce = setTimeout(() => rebuild(w), 100);
			}
		});
		if (element.parentElement) ro.observe(element.parentElement);

		return () => {
			clearTimeout(debounce);
			ro.disconnect();
			cropt?.destroy();
			cropt = null;
		};
	};

	function bindFile(f: File) {
		if (!cropt || boundFile === f) return;
		boundFile = f;
		const url = URL.createObjectURL(f);
		cropt
			.bind(url)
			.catch((err) => {
				console.error('cropt bind failed', err);
				toaster.error({
					title: 'Could not load image',
					description: 'The file may be corrupt or in an unsupported format.'
				});
			})
			.finally(() => URL.revokeObjectURL(url));
	}

	$effect(() => {
		if (cropt) bindFile(file);
	});

	async function upload() {
		if (!cropt) return;
		uploading = true;
		try {
			const blob = await cropt.toBlob(1920, 'image/jpeg', 0.85);
			if (await uploadImage(blob)) {
				toaster.success({ title: 'Image uploaded' });
				onUploaded();
			}
		} catch {
			toaster.error({ title: 'Could not process image' });
		} finally {
			uploading = false;
		}
	}
</script>

<div class={`w-full space-y-3 ${className}`}>
	<!-- Custom properties cascade into cropt's .cr-boundary to size it from the measured width. -->
	<div class="relative" style="height: {cropH + 54}px;">
		<div
			style="--cropt-w: {cropW}px; --cropt-h: {cropH}px"
			{@attach cropperAttachment}
			class="absolute w-full"
		></div>
	</div>

	<div class="flex gap-2">
		<button
			class="btn preset-filled-primary-500"
			data-testid="cropper-upload"
			onclick={upload}
			disabled={uploading}
		>
			{uploading ? 'Uploading…' : 'Upload'}
		</button>
		<button class="btn preset-tonal-surface" onclick={onCancel} disabled={uploading}>
			Cancel
		</button>
	</div>
</div>

<style>
	:global(.cropt-container .cr-boundary) {
		width: var(--cropt-w);
		height: var(--cropt-h);
	}
</style>
