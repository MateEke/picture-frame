<script lang="ts">
	import type { Component } from 'svelte';
	import {
		CloudIcon,
		CloudCheckIcon,
		CloudAlertIcon,
		RefreshCwIcon,
		ExternalLinkIcon
	} from '@lucide/svelte';
	import type { LibrarySync } from '$lib/api/types.gen';
	import { timeAgo } from '$lib/helpers';

	let {
		sync,
		shareUrl,
		syncing,
		onSync
	}: {
		sync: LibrarySync | undefined;
		shareUrl: string | null;
		syncing: boolean;
		onSync: () => void;
	} = $props();

	type Tone = 'success' | 'warning' | 'surface';
	const toneClass: Record<Tone, string> = {
		success: 'preset-tonal-success',
		warning: 'preset-tonal-warning',
		surface: 'preset-tonal-surface'
	};

	interface View {
		icon: Component;
		tone: Tone;
		headline: string;
		detail: string;
	}

	const view = $derived.by((): View => {
		if (!sync || !sync.last_sync) {
			return {
				icon: CloudIcon,
				tone: 'surface',
				headline: 'Immich album',
				detail: 'Waiting for the first sync…'
			};
		}
		if (sync.last_error) {
			return {
				icon: CloudAlertIcon,
				tone: 'warning',
				headline: 'Sync had trouble',
				detail: `Last tried ${timeAgo(sync.last_sync)}`
			};
		}
		return {
			icon: CloudCheckIcon,
			tone: 'success',
			headline: 'Immich album',
			detail: `Synced ${timeAgo(sync.last_sync)}`
		};
	});

	const Icon = $derived(view.icon);
</script>

<div class="card bg-surface-100-900 reveal space-y-4 p-6" data-testid="immich-status">
	<div class="flex items-center gap-4">
		<div class="grid size-12 shrink-0 place-items-center rounded-full {toneClass[view.tone]}">
			<Icon class="size-6" />
		</div>
		<div class="min-w-0 flex-1">
			<p class="truncate text-lg font-semibold">{view.headline}</p>
			<p class="text-surface-500-400 text-sm">{view.detail}</p>
		</div>
		{#if sync}
			<div class="text-right">
				<p class="text-2xl font-semibold tabular-nums">{sync.asset_count}</p>
				<p class="text-surface-500-400 text-xs">photos</p>
			</div>
		{/if}
	</div>

	{#if sync?.last_error}
		<p class="preset-tonal-warning rounded-lg px-3 py-2 text-sm">{sync.last_error}</p>
	{/if}

	<p class="text-surface-500-400 text-sm">
		Photos are managed in your Immich album. Add or remove them there, and the frame keeps in sync.
	</p>

	<div class="border-surface-200-800 flex flex-wrap items-center gap-2 border-t pt-4">
		{#if shareUrl}
			<a
				href={shareUrl}
				target="_blank"
				rel="external noreferrer"
				class="text-primary-500 flex items-center gap-1.5 text-sm hover:underline"
			>
				<ExternalLinkIcon class="size-4" />Open album
			</a>
		{/if}
		<button
			class="btn btn-sm preset-tonal-primary ml-auto flex items-center gap-1.5"
			data-testid="immich-sync"
			onclick={onSync}
			disabled={syncing}
		>
			<RefreshCwIcon class="size-4 {syncing ? 'animate-spin' : ''}" />
			{syncing ? 'Syncing…' : 'Sync now'}
		</button>
	</div>
</div>
