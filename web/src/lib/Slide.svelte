<script lang="ts">
	import type { HTMLAttributes } from 'svelte/elements';

	let {
		images,
		onAllLoad,
		onError,
		testId,
		vertical,
		class: className,
		...rest
	}: {
		images: string[];
		onAllLoad?: () => void;
		onError?: () => void;
		testId?: string;
		vertical?: boolean;
	} & HTMLAttributes<HTMLDivElement> = $props();

	// The kiosk follows its viewport in pure CSS; the dashboard preview, whose
	// orientation is the frame's not the admin window's, sets vertical explicitly.
	function directionClass(v: boolean | undefined): string {
		if (v === undefined) return 'portrait:flex-col';
		return v ? 'flex-col' : 'flex-row';
	}
	const direction = $derived(directionClass(vertical));

	let container: HTMLDivElement;

	// Panes are rendered by the time this effect runs; decode() makes them paint-ready
	// (unlike complete) so the fade shows no black pane. Teardown cancels on slide change.
	$effect(() => {
		void images;
		const panes = [...container.querySelectorAll('img')];
		if (panes.length === 0) return;
		let cancelled = false;
		Promise.all(panes.map((img) => img.decode()))
			.then(() => {
				if (!cancelled) onAllLoad?.();
			})
			.catch(() => {
				if (!cancelled) onError?.();
			});
		return () => {
			cancelled = true;
		};
	});
</script>

<div
	bind:this={container}
	{...rest}
	class={['flex h-full w-full gap-2 bg-black', direction, className]}
>
	{#each images as name, i (i)}
		<img
			src="/img/{name}"
			alt=""
			decoding="async"
			class="min-h-0 min-w-0 flex-1 object-cover"
			data-testid={i === 0 ? testId : undefined}
		/>
	{/each}
</div>
