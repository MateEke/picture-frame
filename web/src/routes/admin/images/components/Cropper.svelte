<script lang="ts">
	import { Cropt } from 'cropt';
	import 'cropt/src/cropt.css';

	import type { Attachment } from 'svelte/attachments';

	import { uploadImage } from '$lib/images';
	import { toaster } from '$lib/toaster';
	import { CROP_RATIOS, getCropRatio, setCropRatio, type CropRatio } from '$lib/uploadPrefs';
	import { fileToJpegBlob } from '$lib/imageProcessing';

	interface Props {
		file: File;
		onUploaded: () => void;
		onCancel: () => void;
		class?: string;
	}

	let { file, onUploaded, onCancel, class: className = '' }: Props = $props();

	// Fixed boundary box; the crop viewport sits inside it per ratio.
	const BOUND_W = 460;
	const BOUND_H = 340;
	const FILL = 0.9; // viewport margin inside the boundary

	let cropt: Cropt | null = null;
	let uploading = $state(false);
	let ratio = $state(getCropRatio());

	let availW = $state(0);
	let boundaryW = $derived(Math.min(availW, BOUND_W));
	// Largest ratio-rect that fits the boundary.
	let viewport = $derived.by(() => {
		const w = Math.round(Math.min(boundaryW, (BOUND_H * ratio.w) / ratio.h) * FILL);
		return { width: w, height: Math.round((w * ratio.h) / ratio.w) };
	});

	const cropperAttachment: Attachment = (element) => {
		if (!(element instanceof HTMLElement)) return;

		const build = () => {
			if (boundaryW === 0) return;
			cropt?.destroy();
			cropt = new Cropt(element, { viewport, mouseWheelZoom: 'ctrl' });
			const url = URL.createObjectURL(file);
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
		};

		let debounce: ReturnType<typeof setTimeout>;
		let first = true;
		let lastW = 0;
		const ro = new ResizeObserver(([entry]) => {
			const w = Math.floor(entry.contentRect.width);
			if (w === 0 || w === lastW) return;
			lastW = w;
			availW = w;
			// Width change rebuilds the boundary; ratio change uses setOptions.
			if (first) {
				first = false;
				build();
			} else {
				clearTimeout(debounce);
				debounce = setTimeout(build, 100);
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

	function selectRatio(r: CropRatio) {
		if (r.id === ratio.id) return;
		ratio = r;
		setCropRatio(r);
		cropt?.setOptions({ viewport });
	}

	async function finishUpload(makeBlob: () => Promise<Blob>) {
		uploading = true;
		try {
			const blob = await makeBlob();
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

	async function upload() {
		if (!cropt) return;
		const c = cropt;
		// toBlob scales the longest side to 1920.
		await finishUpload(() => c.toBlob(1920, 'image/jpeg', 0.85));
	}

	async function uploadOriginal() {
		await finishUpload(() => fileToJpegBlob(file));
	}
</script>

<div class={`w-full space-y-3 ${className}`}>
	<div class="flex flex-wrap justify-center gap-2" role="group" aria-label="Crop ratio">
		{#each CROP_RATIOS as r (r.id)}
			<button
				type="button"
				class={[
					'btn btn-sm',
					r.id === ratio.id ? 'preset-filled-primary-500' : 'preset-tonal-surface'
				]}
				data-testid="crop-ratio-{r.id}"
				aria-pressed={r.id === ratio.id}
				onclick={() => selectRatio(r)}
				disabled={uploading}
			>
				{r.id}
			</button>
		{/each}
	</div>

	<!-- Full-width so cropt centres its boundary; min-height reserves space while it builds. -->
	<div
		{@attach cropperAttachment}
		style="--bound-w: {boundaryW}px; --bound-h: {BOUND_H}px; min-height: {BOUND_H + 50}px"
		class="w-full"
	></div>

	<div class="flex flex-wrap justify-center gap-2">
		<button
			class="btn preset-filled-primary-500"
			data-testid="cropper-upload"
			onclick={upload}
			disabled={uploading}
		>
			{uploading ? 'Uploading…' : 'Upload'}
		</button>
		<button
			class="btn preset-outlined-primary-500"
			data-testid="cropper-upload-original"
			onclick={uploadOriginal}
			disabled={uploading}
		>
			Upload without cropping
		</button>
		<button class="btn preset-tonal-surface" onclick={onCancel} disabled={uploading}>
			Cancel
		</button>
	</div>
</div>

<style>
	/* cropt hardcodes 320px; size the box and match the slider. */
	:global(.cropt-container .cr-boundary),
	:global(.cropt-container .cr-slider-wrap) {
		width: var(--bound-w);
	}
	:global(.cropt-container .cr-boundary) {
		height: var(--bound-h);
	}
</style>
