<script lang="ts">
	import { FolderOpenIcon, HouseIcon, ImagesIcon, SendIcon, SettingsIcon } from '@lucide/svelte';
	import { onDestroy } from 'svelte';
	import { getSSEContext } from '$lib/sse.svelte';
	import { IdleTimer, throttle, wake } from '$lib/touch';
	import Images from '../components/Images.svelte';
	import Overlay from '../components/Overlay.svelte';
	import { fi } from './fi';
	import HomeView from './HomeView.svelte';
	import GalleryView from './GalleryView.svelte';
	import UploadView from './UploadView.svelte';
	import FilesView from './FilesView.svelte';
	import SettingsView from './SettingsView.svelte';

	type Tab = 'home' | 'gallery' | 'upload' | 'files' | 'settings';

	const sse = getSSEContext();

	// Boots into the slideshow: after a reboot or reload the frame shows photos,
	// and a tap brings up the menu.
	let mode = $state<'slideshow' | 'menu'>('slideshow');
	let tab = $state<Tab>('home');

	const idleMs = $derived((sse.kiosk?.sleep?.idle_after_seconds ?? 120) * 1000);
	const screenOff = $derived(sse.screen ? !sse.screen.on : false);

	const idle = new IdleTimer(120_000, () => (mode = 'slideshow'));
	$effect(() => idle.setDelay(idleMs));
	onDestroy(() => idle.stop());

	// Interaction counts as presence for the night window and idle-blank, but a
	// finger scrolling the gallery shouldn't be a request per event.
	const presence = throttle(wake, 30_000);

	function openMenu(to: Tab = 'home') {
		tab = to;
		mode = 'menu';
		idle.poke();
	}

	function startSlideshow() {
		idle.stop();
		mode = 'slideshow';
	}

	function activity() {
		idle.poke();
		presence();
	}

	const tabs = [
		{ id: 'home', label: fi.tabs.home, icon: HouseIcon },
		{ id: 'gallery', label: fi.tabs.gallery, icon: ImagesIcon },
		{ id: 'upload', label: fi.tabs.upload, icon: SendIcon },
		{ id: 'files', label: fi.tabs.files, icon: FolderOpenIcon },
		{ id: 'settings', label: fi.tabs.settings, icon: SettingsIcon }
	] as const;
</script>

{#if mode === 'slideshow'}
	<!-- Only one heavy view is mounted at a time: the menu unmounts with the slideshow up and vice versa. -->
	<div class="h-screen w-screen overflow-hidden" data-testid="touch-slideshow">
		<Images onTap={() => openMenu()} />
		<Overlay />
	</div>
{:else}
	<div
		class="touch-app fixed inset-0 flex flex-col bg-neutral-950 text-neutral-100 select-none"
		data-testid="touch-menu"
		onpointerdowncapture={activity}
		role="application"
	>
		<main class="relative min-h-0 flex-1 overflow-hidden">
			{#if tab === 'home'}
				<HomeView onStart={startSlideshow} />
			{:else if tab === 'gallery'}
				<GalleryView />
			{:else if tab === 'upload'}
				<UploadView />
			{:else if tab === 'files'}
				<FilesView />
			{:else}
				<SettingsView />
			{/if}
		</main>

		<nav
			class="grid shrink-0 grid-cols-5 border-t border-neutral-800 bg-neutral-900"
			data-testid="touch-tabs"
		>
			{#each tabs as t (t.id)}
				{@const Icon = t.icon}
				<button
					type="button"
					class={[
						'flex h-20 flex-col items-center justify-center gap-1 text-base font-medium',
						tab === t.id ? 'text-amber-300' : 'text-neutral-400 active:text-neutral-200'
					]}
					aria-current={tab === t.id ? 'page' : undefined}
					data-testid="touch-tab-{t.id}"
					onclick={() => (tab = t.id)}
				>
					<Icon class="size-8" strokeWidth={tab === t.id ? 2.25 : 1.75} />
					{t.label}
				</button>
			{/each}
		</nav>

		{#if screenOff}
			<!-- Dark panel: swallow the tap that wakes it so it can't hit a button blind. -->
			<button
				type="button"
				class="fixed inset-0 z-50 bg-black"
				aria-label={fi.wakeHint}
				data-testid="touch-wake"
				onclick={wake}
			></button>
		{/if}
	</div>
{/if}

<style>
	/* No hover styles on a touch panel, and no double-tap zoom delay. */
	.touch-app :global(button),
	.touch-app :global([role='button']) {
		touch-action: manipulation;
		-webkit-tap-highlight-color: transparent;
	}
</style>
