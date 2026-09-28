<script lang="ts">
	import {
		ChevronLeftIcon,
		ChevronRightIcon,
		EyeIcon,
		EyeOffIcon,
		Trash2Icon,
		XIcon
	} from '@lucide/svelte';
	import { classifyGesture, type ImageItem } from '$lib/touch';
	import { fi } from './fi';

	let {
		items,
		index,
		onIndex,
		onClose,
		onToggle,
		onDelete
	}: {
		items: ImageItem[];
		index: number;
		onIndex: (i: number) => void;
		onClose: () => void;
		onToggle: (img: ImageItem) => void;
		onDelete: (img: ImageItem) => void;
	} = $props();

	const img = $derived(items[index]);
	const hasPrev = $derived(index > 0);
	const hasNext = $derived(index < items.length - 1);

	// Warm the neighbours so a swipe lands on a decoded photo; only one each way
	// to keep memory flat on the Pi.
	$effect(() => {
		for (const n of [items[index - 1], items[index + 1]]) {
			if (n) new Image().src = `/img/${n.name}`;
		}
	});

	let start: { x: number; y: number; t: number } | null = null;
	function down(e: PointerEvent) {
		if (e.isPrimary) start = { x: e.clientX, y: e.clientY, t: performance.now() };
	}
	function up(e: PointerEvent) {
		if (!start) return;
		const g = classifyGesture(
			e.clientX - start.x,
			e.clientY - start.y,
			performance.now() - start.t
		);
		start = null;
		if (g === 'next' && hasNext) onIndex(index + 1);
		if (g === 'prev' && hasPrev) onIndex(index - 1);
	}
</script>

<div
	class="fixed inset-0 z-30 flex flex-col bg-black"
	data-testid="touch-viewer"
	role="dialog"
	aria-modal="true"
>
	<div
		class="relative min-h-0 flex-1 touch-none"
		role="presentation"
		onpointerdown={down}
		onpointerup={up}
		onpointercancel={() => (start = null)}
	>
		{#key img.name}
			<img
				src="/img/{img.name}"
				alt=""
				decoding="async"
				draggable="false"
				class="absolute inset-0 h-full w-full object-contain"
				data-testid="touch-viewer-img"
			/>
		{/key}
		{#if hasPrev}
			<button
				type="button"
				class="absolute top-1/2 left-3 flex size-20 -translate-y-1/2 items-center justify-center rounded-full bg-black/50"
				aria-label={fi.gallery.prev}
				onclick={() => onIndex(index - 1)}><ChevronLeftIcon class="size-12" /></button
			>
		{/if}
		{#if hasNext}
			<button
				type="button"
				class="absolute top-1/2 right-3 flex size-20 -translate-y-1/2 items-center justify-center rounded-full bg-black/50"
				aria-label={fi.gallery.next}
				data-testid="touch-viewer-next"
				onclick={() => onIndex(index + 1)}><ChevronRightIcon class="size-12" /></button
			>
		{/if}
		<button
			type="button"
			class="absolute top-4 right-4 flex size-20 items-center justify-center rounded-full bg-black/60"
			aria-label={fi.gallery.close}
			data-testid="touch-viewer-close"
			onclick={onClose}><XIcon class="size-12" /></button
		>
		<span class="absolute top-6 left-6 rounded-xl bg-black/60 px-4 py-2 text-xl tabular-nums"
			>{index + 1} / {items.length}</span
		>
	</div>
	<div class="grid shrink-0 grid-cols-2 gap-3 bg-neutral-950 p-3">
		<button
			type="button"
			class="flex h-20 items-center justify-center gap-3 rounded-2xl bg-neutral-800 text-xl active:bg-neutral-700"
			data-testid="touch-viewer-toggle"
			onclick={() => onToggle(img)}
		>
			{#if img.included}
				<EyeOffIcon class="size-8" />{fi.gallery.hide}
			{:else}
				<EyeIcon class="size-8" />{fi.gallery.show}
			{/if}
		</button>
		<button
			type="button"
			class="flex h-20 items-center justify-center gap-3 rounded-2xl bg-red-900 text-xl active:bg-red-800"
			data-testid="touch-viewer-delete"
			onclick={() => onDelete(img)}
		>
			<Trash2Icon class="size-8" />{fi.gallery.delete}
		</button>
	</div>
</div>
