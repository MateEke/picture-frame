<script lang="ts">
	import { MinusIcon, PlusIcon } from '@lucide/svelte';

	let {
		label,
		value,
		onStep,
		canDown = true,
		canUp = true,
		testId
	}: {
		label: string;
		value: string;
		onStep: (dir: 1 | -1) => void;
		canDown?: boolean;
		canUp?: boolean;
		testId?: string;
	} = $props();
</script>

<div class="flex items-center gap-4 py-3" data-testid={testId}>
	<span class="min-w-0 flex-1 text-xl leading-snug">{label}</span>
	<button
		type="button"
		class="flex size-16 shrink-0 items-center justify-center rounded-2xl bg-neutral-800 active:bg-neutral-700 disabled:opacity-30"
		disabled={!canDown}
		aria-label="−"
		data-testid={testId ? `${testId}-down` : undefined}
		onclick={() => onStep(-1)}><MinusIcon class="size-8" /></button
	>
	<span
		class="w-32 shrink-0 text-center text-2xl font-semibold tabular-nums"
		data-testid={testId ? `${testId}-value` : undefined}>{value}</span
	>
	<button
		type="button"
		class="flex size-16 shrink-0 items-center justify-center rounded-2xl bg-neutral-800 active:bg-neutral-700 disabled:opacity-30"
		disabled={!canUp}
		aria-label="+"
		data-testid={testId ? `${testId}-up` : undefined}
		onclick={() => onStep(1)}><PlusIcon class="size-8" /></button
	>
</div>
