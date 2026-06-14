<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { PageProps } from './$types';
	import { loadWiFiStatus, scanNetworks, connectWiFi, forgetNetwork } from '$lib/wifi';
	import type { WiFiState, WiFiNetwork } from '$lib/api/types.gen';
	import { toaster } from '$lib/toaster';
	import ConnectBanner, { type ConnectBannerState } from './components/ConnectBanner.svelte';
	import StatusCard from './components/StatusCard.svelte';
	import NetworkList from './components/NetworkList.svelte';
	import ConnectDialog from './components/ConnectDialog.svelte';
	import ForgetDialog from './components/ForgetDialog.svelte';
	import APFallback from './components/APFallback.svelte';

	let { data }: PageProps = $props();

	// polledStatus is updated by the poll loop; falls back to SSR data until the first poll lands.
	let polledStatus = $state<WiFiState | null>(null);
	let status = $derived(polledStatus ?? data.status);
	let networks = $state<WiFiNetwork[]>([]);
	let scanning = $state(false);
	let showConnectDialog = $state(false);
	let selectedNetwork = $state<WiFiNetwork | null>(null);
	let pollTimer: ReturnType<typeof setTimeout> | undefined;
	let pollAbort: AbortController | undefined;
	let polling = false;

	// Persistent banner for a connect attempt; stays 'connecting' until polling
	// resolves it to success/error, or 'unconfirmed' if we lose contact.
	let connectBanner = $state<ConnectBannerState | null>(null);
	let bannerTimer: ReturnType<typeof setTimeout> | undefined;
	const isConnecting = $derived(connectBanner?.phase === 'connecting');

	// Pending Forget confirmation; active flags forgetting the live connection.
	let forgetTarget = $state<{ ssid: string; active: boolean } | null>(null);

	onMount(() => {
		polling = true;
		pollLoop();
	});

	onDestroy(() => {
		polling = false;
		clearTimeout(pollTimer);
		clearTimeout(bannerTimer);
		pollAbort?.abort();
	});

	// Self-scheduling: next request queued only after current one settles (no overlap).
	async function pollLoop() {
		await pollStatus();
		if (polling) pollTimer = setTimeout(pollLoop, 3000);
	}

	async function pollStatus() {
		pollAbort?.abort();
		pollAbort = new AbortController();
		const signal = AbortSignal.any([pollAbort.signal, AbortSignal.timeout(5000)]);
		const latest = await loadWiFiStatus(fetch, signal).catch(() => null);
		if (latest !== null) polledStatus = latest;
		updateConnectBanner(latest);
	}

	// setBanner centralises the auto-dismiss timer so a stale 'success' banner
	// doesn't linger; errors/unconfirmed stay until the user dismisses them.
	function setBanner(next: typeof connectBanner) {
		clearTimeout(bannerTimer);
		connectBanner = next;
		if (next?.phase === 'success') {
			bannerTimer = setTimeout(() => {
				if (connectBanner?.phase === 'success') connectBanner = null;
			}, 5000);
		}
	}

	function updateConnectBanner(latest: WiFiState | null) {
		const banner = connectBanner;
		if (!banner || banner.phase !== 'connecting') return;
		if (latest) {
			if (latest.mode === 'connected' && latest.ssid === banner.ssid) {
				setBanner({ ...banner, phase: 'success' });
				// Connecting saved a profile, so mark it Known (moves it to Saved) now.
				networks = networks.map((n) => (n.ssid === banner.ssid ? { ...n, known: true } : n));
				return;
			}
			if (latest.last_connect_error && latest.last_connect_ssid === banner.ssid) {
				const known = networks.some((n) => n.ssid === banner.ssid && n.known);
				setBanner({ ...banner, phase: 'error', known, message: latest.last_connect_error });
				return;
			}
		}
		// No definitive result yet; after ~45s assume we've lost contact.
		if (Date.now() - banner.startedAt > 45000) {
			setBanner({ ...banner, phase: 'unconfirmed' });
		}
	}

	async function handleScan() {
		scanning = true;
		try {
			networks = await scanNetworks();
		} catch {
			toaster.error({ title: 'Scan failed', description: 'Could not retrieve network list.' });
		} finally {
			scanning = false;
		}
	}

	// Known and open networks connect in one click; only unknown secured networks open the dialog.
	function startConnect(net: WiFiNetwork) {
		if (net.known || net.security === '') {
			connectToNetwork(net.ssid, net.known);
		} else {
			selectedNetwork = net;
			showConnectDialog = true;
		}
	}

	async function connectToNetwork(ssid: string, known: boolean, password = '') {
		showConnectDialog = false;
		setBanner({ ssid, phase: 'connecting', known, startedAt: Date.now() });
		const ok = await connectWiFi(ssid, password);
		if (!ok) {
			// connectWiFi already toasted the cause (busy / network error).
			setBanner({
				ssid,
				phase: 'error',
				known,
				message: 'Could not start the connection. Please try again.',
				startedAt: Date.now()
			});
		}
		// On success (202) the banner stays 'connecting' until polling resolves it.
	}

	function askForget(net: WiFiNetwork) {
		forgetTarget = {
			ssid: net.ssid,
			active: status?.mode === 'connected' && net.ssid === status.ssid
		};
	}

	// Frame fast-tracks to another known network before raising AP; only promise
	// the hotspot when the scan shows no other known network in range.
	function forgetActiveOutcome(ssid: string): string {
		const ap = status?.ap_enabled ? (status.ap_ssid ?? '') : '';
		const otherKnownInRange = networks.some((n) => n.known && n.ssid !== ssid);
		const scannedNoOtherKnown = networks.length > 0 && !otherKnownInRange;
		if (ap && scannedNoOtherKnown) {
			return `The ${ap} hotspot will appear shortly. Connect to it to keep configuring.`;
		}
		if (ap) {
			return `The frame will reconnect to another saved network if one is in range, otherwise the ${ap} hotspot will appear.`;
		}
		return 'The frame will reconnect to another saved network if one is in range, otherwise it stays unreachable until a known network returns.';
	}

	async function confirmForget() {
		if (!forgetTarget) return;
		const { ssid, active } = forgetTarget;
		forgetTarget = null;

		if (active) {
			// Response usually never arrives (link drops); fire and set expectations.
			void forgetNetwork(ssid);
			toaster.info({
				title: `Disconnecting from ${ssid}`,
				description: forgetActiveOutcome(ssid)
			});
			return;
		}

		const ok = await forgetNetwork(ssid);
		if (ok) {
			toaster.success({ title: 'Network forgotten', description: ssid });
			networks = networks.map((n) => (n.ssid === ssid ? { ...n, known: false } : n));
		} else {
			toaster.error({ title: 'Forget failed', description: ssid });
		}
	}
</script>

<div class="space-y-6">
	<h1 class="h2">WiFi</h1>

	{#if status === null}
		<div class="card bg-surface-100-900 p-6">
			<p class="text-surface-500-400" data-testid="wifi-unavailable">
				WiFi management not available in this environment.
			</p>
		</div>
	{:else}
		{#if connectBanner}
			<ConnectBanner
				banner={connectBanner}
				hostname={status.hostname}
				ondismiss={() => (connectBanner = null)}
			/>
		{/if}

		<StatusCard {status} />

		<NetworkList
			{networks}
			{status}
			{scanning}
			{isConnecting}
			onscan={handleScan}
			onconnect={startConnect}
			onforget={askForget}
		/>

		<APFallback {status} onsave={(newState) => (polledStatus = newState)} />
	{/if}
</div>

{#if showConnectDialog && selectedNetwork}
	<ConnectDialog
		network={selectedNetwork}
		onconnect={(ssid, password) => connectToNetwork(ssid, false, password)}
		oncancel={() => (showConnectDialog = false)}
	/>
{/if}

{#if forgetTarget}
	<ForgetDialog
		target={forgetTarget}
		outcome={forgetActiveOutcome(forgetTarget.ssid)}
		onconfirm={confirmForget}
		oncancel={() => (forgetTarget = null)}
	/>
{/if}
