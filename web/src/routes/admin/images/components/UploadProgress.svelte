<script lang="ts">
	import { CloudUploadIcon } from '@lucide/svelte';

	let { done, total, onStop }: { done: number; total: number; onStop: () => void } = $props();

	const current = $derived(Math.min(done + 1, total));
	const percent = $derived(total === 0 ? 0 : Math.round((done / total) * 100));
</script>

<div
	class="border-surface-300-700 flex flex-col items-center gap-1.5 rounded-lg border-2 p-4 text-center sm:gap-2 sm:p-8"
	data-testid="bulk-upload-progress"
>
	<div
		class="bg-primary-500/10 text-primary-500 grid size-9 place-items-center rounded-full sm:size-12"
	>
		<CloudUploadIcon class="size-5 motion-safe:animate-pulse sm:size-6" />
	</div>

	<p class="font-medium" aria-live="polite" data-testid="bulk-upload-count">
		Adding photo {current} of {total}
	</p>

	<div
		class="bg-surface-300-700 h-1.5 w-full max-w-xs overflow-hidden rounded-full"
		role="progressbar"
		aria-label="Adding photos"
		aria-valuenow={done}
		aria-valuemin={0}
		aria-valuemax={total}
	>
		<div
			class="bg-primary-500 h-full motion-safe:transition-[width] motion-safe:duration-200"
			style="width: {percent}%"
		></div>
	</div>

	<button
		type="button"
		class="btn btn-sm preset-outlined-surface-500 mt-2"
		data-testid="bulk-upload-stop"
		onclick={onStop}
	>
		Stop
	</button>
</div>
