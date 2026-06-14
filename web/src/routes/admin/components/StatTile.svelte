<script lang="ts">
	import type { Component } from 'svelte';
	import type { HTMLAttributes } from 'svelte/elements';
	import type { ResolvedPathname } from '$app/types';

	type Tone = 'primary' | 'success' | 'warning' | 'surface';

	let {
		label,
		value,
		Icon,
		href,
		sub,
		tone = 'primary',
		...rest
	}: {
		label: string;
		value: string;
		Icon: Component;
		href?: ResolvedPathname;
		sub?: string;
		tone?: Tone;
	} & HTMLAttributes<HTMLElement> = $props();

	const toneClass: Record<Tone, string> = {
		primary: 'text-primary-500',
		success: 'text-success-500',
		warning: 'text-warning-500',
		surface: 'text-surface-500'
	};
</script>

{#snippet body()}
	<Icon class="{toneClass[tone]} size-6 shrink-0" />
	<div class="min-w-0">
		<p class="text-surface-500-400 text-xs">{label}</p>
		<p class="truncate font-medium">{value}</p>
		{#if sub}<p class="text-surface-500-400 truncate text-xs">{sub}</p>{/if}
	</div>
{/snippet}

{#if href}
	<a
		{...rest}
		{href}
		class="card bg-surface-100-900 hover:bg-surface-200-800 flex items-center gap-3 p-4 transition-colors"
	>
		{@render body()}
	</a>
{:else}
	<div {...rest} class="card bg-surface-100-900 flex items-center gap-3 p-4">
		{@render body()}
	</div>
{/if}
