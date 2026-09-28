<script lang="ts">
	import { CheckIcon, EyeIcon, EyeOffIcon, Trash2Icon } from '@lucide/svelte';
	import { onDestroy } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { getSSEContext } from '$lib/sse.svelte';
	import { fetchImages, removeImages, setIncluded, visibleRows, type ImageItem } from '$lib/touch';
	import { fi } from './fi';
	import ConfirmSheet from './ConfirmSheet.svelte';
	import Notice from './Notice.svelte';
	import PhotoViewer from './PhotoViewer.svelte';

	type Filter = 'all' | 'shown' | 'hidden';

	const sse = getSSEContext();

	let images = $state<ImageItem[] | null>(null);
	let filter = $state<Filter>('all');
	let selecting = $state(false);
	const selected = new SvelteSet<string>();
	let viewerIndex = $state<number | null>(null);
	let confirmDelete = $state<string[] | null>(null);
	let notice = $state<{ text: string; tone: 'info' | 'error' } | null>(null);
	let noticeTimer: ReturnType<typeof setTimeout> | undefined;

	function flash(text: string, tone: 'info' | 'error' = 'info') {
		notice = { text, tone };
		clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = null), 3000);
	}
	onDestroy(() => clearTimeout(noticeTimer));

	// Refetch on every library change (uploads from a phone appear live).
	$effect(() => {
		void sse.libraryRev;
		fetchImages().then((imgs) => {
			if (imgs) images = imgs;
		});
	});

	const visible = $derived(
		(images ?? []).filter((i) =>
			filter === 'all' ? true : filter === 'shown' ? i.included : !i.included
		)
	);
	const allHidden = $derived(!!images?.length && images.every((i) => !i.included));

	// --- Virtual grid: only the rows in view (plus overscan) are in the DOM, so a
	// library of thousands scrolls like one of ten on the Pi.
	let scroller = $state<HTMLDivElement>();
	let width = $state(0);
	let height = $state(0);
	let scrollTop = $state(0);
	let raf = 0;
	const gap = 6;
	const cols = $derived(width >= 1000 ? 5 : width >= 640 ? 3 : 2);
	const cell = $derived(cols ? (width - gap * (cols - 1)) / cols : 0);
	const rowH = $derived(cell + gap);
	const rowCount = $derived(Math.ceil(visible.length / cols));
	const win = $derived(visibleRows(scrollTop, height, rowH, rowCount));
	const slice = $derived(
		visible.slice(win.firstRow * cols, win.lastRow * cols).map((img, k) => {
			const i = win.firstRow * cols + k;
			return { img, i, x: (i % cols) * (cell + gap), y: Math.floor(i / cols) * rowH };
		})
	);

	function onScroll() {
		if (raf) return;
		raf = requestAnimationFrame(() => {
			raf = 0;
			if (scroller) scrollTop = scroller.scrollTop;
		});
	}
	onDestroy(() => cancelAnimationFrame(raf));

	// Reset scroll when the filter changes so the grid doesn't open mid-nowhere.
	$effect(() => {
		void filter;
		if (scroller) scroller.scrollTop = 0;
		scrollTop = 0;
	});

	// --- Long press starts selection.
	let pressTimer: ReturnType<typeof setTimeout> | undefined;
	let pressOrigin: { x: number; y: number } | null = null;
	let longPressed = false;

	function pressStart(e: PointerEvent, name: string) {
		longPressed = false;
		pressOrigin = { x: e.clientX, y: e.clientY };
		clearTimeout(pressTimer);
		pressTimer = setTimeout(() => {
			longPressed = true;
			selecting = true;
			selected.add(name);
		}, 500);
	}
	function pressMove(e: PointerEvent) {
		if (pressOrigin && Math.hypot(e.clientX - pressOrigin.x, e.clientY - pressOrigin.y) > 10) {
			clearTimeout(pressTimer);
			pressOrigin = null;
		}
	}
	function pressEnd() {
		clearTimeout(pressTimer);
		pressOrigin = null;
	}

	function tapCell(i: number, name: string) {
		if (longPressed) {
			longPressed = false;
			return;
		}
		if (selecting) {
			if (selected.has(name)) selected.delete(name);
			else selected.add(name);
			return;
		}
		viewerIndex = i;
	}

	function endSelection() {
		selecting = false;
		selected.clear();
	}

	async function applyIncluded(names: string[], included: boolean) {
		// Optimistic: flip locally, the library event refetch confirms.
		const prev = images;
		images = (images ?? []).map((i) => (names.includes(i.name) ? { ...i, included } : i));
		if (!(await setIncluded(names, included))) {
			images = prev;
			flash(fi.gallery.saveFailed, 'error');
		}
	}

	async function doDelete(names: string[]) {
		confirmDelete = null;
		const failed = await removeImages(names);
		if (failed > 0) flash(fi.gallery.deleteFailed(failed), 'error');
		const fresh = await fetchImages();
		if (fresh) images = fresh;
		endSelection();
		// Stay in the viewer on the photo that slid into the deleted one's place.
		if (viewerIndex !== null && viewerIndex >= visible.length) {
			viewerIndex = visible.length ? visible.length - 1 : null;
		}
	}

	const filters: { id: Filter; label: string }[] = [
		{ id: 'all', label: fi.gallery.filterAll },
		{ id: 'shown', label: fi.gallery.filterShown },
		{ id: 'hidden', label: fi.gallery.filterHidden }
	];
</script>

<div class="flex h-full flex-col" data-testid="touch-gallery">
	<header class="flex shrink-0 items-center gap-3 border-b border-neutral-800 px-4 py-3">
		{#if selecting}
			<span class="flex-1 text-2xl font-semibold" data-testid="touch-gallery-selected"
				>{fi.gallery.selected(selected.size)}</span
			>
			<button
				type="button"
				class="h-16 rounded-2xl bg-neutral-800 px-5 text-xl active:bg-neutral-700"
				onclick={() => visible.forEach((i) => selected.add(i.name))}>{fi.gallery.selectAll}</button
			>
			<button
				type="button"
				class="h-16 rounded-2xl bg-neutral-800 px-5 text-xl active:bg-neutral-700"
				data-testid="touch-gallery-cancel"
				onclick={endSelection}>{fi.gallery.cancel}</button
			>
		{:else}
			<div class="flex flex-1 gap-2 overflow-x-auto">
				{#each filters as f (f.id)}
					<button
						type="button"
						class={[
							'h-16 shrink-0 rounded-2xl px-5 text-xl font-medium',
							filter === f.id
								? 'bg-amber-400 text-neutral-950'
								: 'bg-neutral-800 active:bg-neutral-700'
						]}
						data-testid="touch-gallery-filter-{f.id}"
						onclick={() => (filter = f.id)}>{f.label}</button
					>
				{/each}
			</div>
			<button
				type="button"
				class="h-16 shrink-0 rounded-2xl bg-neutral-800 px-5 text-xl font-medium active:bg-neutral-700"
				data-testid="touch-gallery-select"
				onclick={() => (selecting = true)}>{fi.gallery.select}</button
			>
		{/if}
	</header>

	{#if allHidden}
		<p class="shrink-0 bg-amber-950 px-5 py-3 text-lg text-amber-200">{fi.gallery.allHiddenNote}</p>
	{/if}

	<div
		bind:this={scroller}
		bind:clientWidth={width}
		bind:clientHeight={height}
		class="relative min-h-0 flex-1 overflow-y-auto overscroll-contain p-0"
		onscroll={onScroll}
	>
		{#if images && images.length === 0}
			<p class="p-8 text-2xl leading-relaxed text-neutral-400">{fi.gallery.empty}</p>
		{:else}
			<div class="relative" style="height: {rowCount * rowH}px">
				{#each slice as { img, i, x, y } (img.name)}
					<button
						type="button"
						class="absolute top-0 left-0 overflow-hidden bg-neutral-900"
						style="width: {cell}px; height: {cell}px; transform: translate3d({x}px, {y}px, 0)"
						data-testid="touch-gallery-item"
						data-name={img.name}
						data-included={img.included ? 'true' : 'false'}
						onpointerdown={(e) => pressStart(e, img.name)}
						onpointermove={pressMove}
						onpointerup={pressEnd}
						onpointercancel={pressEnd}
						oncontextmenu={(e) => e.preventDefault()}
						onclick={() => tapCell(i, img.name)}
					>
						<img
							src="/thumb/{img.name}"
							alt=""
							loading="lazy"
							decoding="async"
							draggable="false"
							class={['h-full w-full object-cover', !img.included && 'opacity-40']}
						/>
						{#if !img.included}
							<span
								class="absolute bottom-2 left-2 flex items-center gap-1 rounded-lg bg-black/70 px-2 py-1 text-sm"
							>
								<EyeOffIcon class="size-4" />
								{fi.gallery.hiddenBadge}
							</span>
						{/if}
						{#if selecting}
							<span
								class={[
									'absolute top-2 right-2 flex size-10 items-center justify-center rounded-full border-2',
									selected.has(img.name)
										? 'border-amber-400 bg-amber-400 text-neutral-950'
										: 'border-white bg-black/40'
								]}
							>
								{#if selected.has(img.name)}<CheckIcon class="size-6" strokeWidth={3} />{/if}
							</span>
						{/if}
					</button>
				{/each}
			</div>
		{/if}
	</div>

	{#if selecting}
		<footer class="grid shrink-0 grid-cols-3 gap-3 border-t border-neutral-800 bg-neutral-900 p-3">
			<button
				type="button"
				disabled={selected.size === 0}
				class="flex h-20 flex-col items-center justify-center gap-1 rounded-2xl bg-neutral-800 text-lg active:bg-neutral-700 disabled:opacity-40"
				data-testid="touch-gallery-show"
				onclick={() => applyIncluded([...selected], true).then(endSelection)}
			>
				<EyeIcon class="size-7" />{fi.gallery.show}
			</button>
			<button
				type="button"
				disabled={selected.size === 0}
				class="flex h-20 flex-col items-center justify-center gap-1 rounded-2xl bg-neutral-800 text-lg active:bg-neutral-700 disabled:opacity-40"
				data-testid="touch-gallery-hide"
				onclick={() => applyIncluded([...selected], false).then(endSelection)}
			>
				<EyeOffIcon class="size-7" />{fi.gallery.hide}
			</button>
			<button
				type="button"
				disabled={selected.size === 0}
				class="flex h-20 flex-col items-center justify-center gap-1 rounded-2xl bg-red-900 text-lg active:bg-red-800 disabled:opacity-40"
				data-testid="touch-gallery-delete"
				onclick={() => (confirmDelete = [...selected])}
			>
				<Trash2Icon class="size-7" />{fi.gallery.delete}
			</button>
		</footer>
	{/if}

	{#if viewerIndex !== null && visible[viewerIndex]}
		<PhotoViewer
			items={visible}
			index={viewerIndex}
			onIndex={(i) => (viewerIndex = i)}
			onClose={() => (viewerIndex = null)}
			onToggle={(img) => applyIncluded([img.name], !img.included)}
			onDelete={(img) => (confirmDelete = [img.name])}
		/>
	{/if}

	{#if confirmDelete}
		<ConfirmSheet
			message={fi.gallery.confirmDelete(confirmDelete.length)}
			confirmLabel={fi.gallery.delete}
			cancelLabel={fi.gallery.cancel}
			onConfirm={() => confirmDelete && doDelete(confirmDelete)}
			onCancel={() => (confirmDelete = null)}
		/>
	{/if}

	{#if notice}
		<Notice text={notice.text} tone={notice.tone} />
	{/if}
</div>
