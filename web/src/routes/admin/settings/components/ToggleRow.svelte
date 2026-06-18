<script lang="ts">
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import { Undo2Icon } from '@lucide/svelte';

	let {
		label,
		checked,
		changed,
		onchange,
		onrevert,
		testId
	}: {
		label: string;
		checked: boolean;
		changed: boolean;
		onchange: (value: boolean) => void;
		onrevert: () => void;
		testId?: string;
	} = $props();
</script>

<div class="flex items-center justify-between">
	<span class="label-text flex items-center gap-1.5">
		{label}
		{#if changed}<span class="bg-primary-500 size-1.5 rounded-full" title="Changed"></span>{/if}
	</span>
	<div class="flex items-center gap-2">
		{#if changed}
			<button
				type="button"
				class="text-surface-500 hover:text-primary-500"
				onclick={onrevert}
				aria-label="Revert {label}"
			>
				<Undo2Icon class="size-3.5" />
			</button>
		{/if}
		<Switch {checked} onCheckedChange={(e) => onchange(e.checked)} data-testid={testId}>
			<Switch.HiddenInput />
			<Switch.Control><Switch.Thumb /></Switch.Control>
		</Switch>
	</div>
</div>
