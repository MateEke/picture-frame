<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Undo2Icon } from '@lucide/svelte';
	import InfoTip from '$lib/InfoTip.svelte';

	let {
		label,
		help,
		changed = false,
		error,
		onrevert,
		trailing,
		children,
		class: className = ''
	}: {
		label: string;
		help?: string;
		changed?: boolean;
		error?: string;
		onrevert?: () => void;
		trailing?: Snippet;
		children: Snippet;
		class?: string;
	} = $props();
</script>

<div class="space-y-1 {className}">
	<div class="flex min-h-6 items-center justify-between gap-2">
		<span class="label-text flex items-center gap-1.5">
			{label}
			{#if help}<InfoTip text={help} />{/if}
			{#if changed}
				<span class="bg-primary-500 size-1.5 rounded-full" title="Changed"></span>
			{/if}
		</span>
		<div class="flex items-center gap-2">
			{#if trailing}{@render trailing()}{/if}
			{#if changed && onrevert}
				<button
					type="button"
					class="text-surface-500 hover:text-primary-500"
					onclick={onrevert}
					aria-label="Revert {label}"
				>
					<Undo2Icon class="size-3.5" />
				</button>
			{/if}
		</div>
	</div>
	{@render children()}
	{#if error}<p class="text-error-500 text-xs">{error}</p>{/if}
</div>
