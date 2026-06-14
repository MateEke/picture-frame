<script lang="ts">
	import '$lib/admin-theme.css';

	import {
		HouseIcon,
		ImagesIcon,
		SettingsIcon,
		FrameIcon,
		WifiIcon,
		LogOutIcon
	} from '@lucide/svelte';
	import { Navigation, Toast } from '@skeletonlabs/skeleton-svelte';
	import { page } from '$app/state';

	import type { LayoutProps } from './$types';
	import { SSESubscriber, setSSEContext } from '$lib/sse.svelte';
	import { reloadOnBackendVersionChange } from '$lib/versionReload.svelte';
	import { toaster } from '$lib/toaster';
	import { logout } from '$lib/auth';

	const sse = new SSESubscriber();
	setSSEContext(sse);
	// Version/update status comes from the page load, so an open tab would otherwise stay stale
	// after a self-update.
	reloadOnBackendVersionChange(() => sse.kiosk?.version);

	let { children, data }: LayoutProps = $props();

	async function doLogout() {
		await logout();
		// Full navigation so the cleared cookie takes effect everywhere.
		location.assign('/login');
	}

	function isActive(href: string): boolean {
		const path = page.url.pathname;
		// /admin matches only itself; sub-routes match their own subtree.
		return href === '/admin' ? path === '/admin' : path === href || path.startsWith(href + '/');
	}

	const links = [
		{ label: 'Dashboard', href: '/admin', icon: HouseIcon },
		{ label: 'Images', href: '/admin/images', icon: ImagesIcon },
		{ label: 'WiFi', href: '/admin/wifi', icon: WifiIcon },
		{ label: 'Settings', href: '/admin/settings', icon: SettingsIcon }
	];
</script>

<!-- Window is the scroll root (navs are fixed); main's padding clears them. -->
<main class="min-h-svh p-4 pb-20 md:pb-4 md:pl-24">
	{@render children()}
</main>
<Navigation layout="bar" class="fixed inset-x-0 bottom-0 z-40 md:hidden" data-testid="nav-bottom">
	<Navigation.Menu class="grid gap-2 {data.auth?.required ? 'grid-cols-5' : 'grid-cols-4'}">
		{#each links as link (link)}
			{const Icon = link.icon}
			<Navigation.TriggerAnchor
				href={link.href}
				class={isActive(link.href) ? 'text-primary-500' : ''}
				data-testid="nav-bottom-link-{link.label}"
			>
				<Icon class="size-5" />
				<Navigation.TriggerText>{link.label}</Navigation.TriggerText>
			</Navigation.TriggerAnchor>
		{/each}
		{#if data.auth?.required}
			<Navigation.Trigger
				onclick={doLogout}
				title="Log out"
				aria-label="Log out"
				data-testid="logout-bar"
			>
				<LogOutIcon class="size-5" />
				<Navigation.TriggerText>Logout</Navigation.TriggerText>
			</Navigation.Trigger>
		{/if}
	</Navigation.Menu>
</Navigation>
<Navigation
	layout="rail"
	class="fixed top-0 bottom-0 left-0 z-40 w-20 max-md:hidden"
	data-testid="nav-rail"
>
	<Navigation.Header>
		<Navigation.TriggerAnchor href="/admin" title="Admin" aria-label="Admin">
			<FrameIcon class="size-8" />
		</Navigation.TriggerAnchor>
	</Navigation.Header>
	<Navigation.Content>
		<Navigation.Menu>
			{#each links as link (link)}
				{const Icon = link.icon}
				<Navigation.TriggerAnchor
					href={link.href}
					class={isActive(link.href) ? 'text-primary-500' : ''}
					data-testid="nav-rail-link-{link.label}"
				>
					<Icon class="size-5" />
					<Navigation.TriggerText>{link.label}</Navigation.TriggerText>
				</Navigation.TriggerAnchor>
			{/each}
		</Navigation.Menu>
	</Navigation.Content>
	{#if data.auth?.required}
		<Navigation.Footer>
			<Navigation.Trigger
				onclick={doLogout}
				title="Log out"
				aria-label="Log out"
				data-testid="logout-rail"
			>
				<LogOutIcon class="size-5" />
				<Navigation.TriggerText>Logout</Navigation.TriggerText>
			</Navigation.Trigger>
		</Navigation.Footer>
	{/if}
</Navigation>

<Toast.Group {toaster}>
	{#snippet children(toast)}
		<Toast {toast}>
			{#if toast.title}<Toast.Title>{toast.title}</Toast.Title>{/if}
			{#if toast.description}<Toast.Description>{toast.description}</Toast.Description>{/if}
		</Toast>
	{/snippet}
</Toast.Group>
