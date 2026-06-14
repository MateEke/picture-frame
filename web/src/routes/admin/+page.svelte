<script lang="ts">
	import type { PageProps } from './$types';
	import type { WiFiState } from '$lib/api/types.gen';
	import { onMount } from 'svelte';
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import {
		WifiIcon,
		WifiOffIcon,
		CloudSunIcon,
		RadioIcon,
		ImagesIcon,
		CloudAlertIcon,
		MonitorIcon,
		ServerIcon,
		RotateCcwIcon,
		InfoIcon
	} from '@lucide/svelte';
	import { resolve } from '$app/paths';
	import { isSensorStale } from '$lib/helpers';
	import { formatDuration } from '$lib/duration';
	import { setScreen } from '$lib/screen';
	import { getSSEContext } from '$lib/sse.svelte';
	import NowPlaying from './components/NowPlaying.svelte';
	import StatTile from './components/StatTile.svelte';
	import SensorReadings from './components/SensorReadings.svelte';
	import AboutModal from './components/AboutModal.svelte';
	import UpdatePanel from './components/UpdatePanel.svelte';
	import RestartConfirmDialog from './settings/components/RestartConfirmDialog.svelte';

	let { data }: PageProps = $props();
	const sse = getSSEContext();

	let now = $state(Date.now());
	onMount(() => {
		const id = setInterval(() => (now = Date.now()), 1_000);
		return () => clearInterval(id);
	});

	// Live screen state comes from SSE; data.screen is the pre-connect fallback. The
	// toggle sets intent (auto vs forced-off); live power also moves on idle-blank.
	const screenAuto = $derived(sse.screen?.auto ?? data.screen?.auto ?? true);
	const screenLiveOn = $derived(sse.screen?.on ?? data.screen?.state === 'on');
	let toggleScreenBusy = $state(false);
	let showRestartDialog = $state(false);
	let showAbout = $state(false);

	const version = $derived(data.system?.version ?? data.update?.current ?? '—');

	function screenReason(auto: boolean, on: boolean): string {
		if (!auto) return 'Turned off manually.';
		if (on) return 'Showing photos.';
		return 'Asleep. Wakes on motion or activity.';
	}

	async function handleToggle(checked: boolean) {
		toggleScreenBusy = true;
		await setScreen(checked);
		toggleScreenBusy = false;
	}

	function wifiTile(w: WiFiState | null): { value: string; sub?: string } {
		if (!w) return { value: 'Unavailable' };
		if (w.mode === 'ap') return { value: w.ap_ssid || 'Hotspot', sub: 'Hotspot mode' };
		if (w.mode === 'connecting') return { value: 'Connecting…' };
		if (w.mode === 'connected') return { value: w.ssid || 'Connected', sub: 'Connected' };
		return { value: 'Disconnected' };
	}
	const wifi = $derived(wifiTile(data.wifi));

	const weather = $derived.by(() => {
		const w = sse.weather;
		if (!w) return { value: 'No data' };
		return { value: `${w.temp.toFixed(1)} °C`, sub: `${w.humidity.toFixed(0)}% humidity` };
	});

	const sensorCount = $derived(Object.keys(sse.sensors).length);
	const staleCount = $derived(
		Object.values(sse.sensors).filter((s) => isSensorStale(s.timestamp)).length
	);
	function sensorSub(count: number, stale: number): string {
		if (count === 0) return 'none yet';
		if (stale > 0) return `${stale} stale`;
		return 'all fresh';
	}

	const libraryValue = $derived(
		data.config?.library.backend === 'immich' ? 'Immich' : 'Local photos'
	);
	// Flag a failing remote sync on the dashboard.
	const librarySyncIssue = $derived(Boolean(data.library?.sync?.last_error));
	const restartPending = $derived(data.config?.restart_pending ?? false);
	const uptime = $derived(data.system ? formatDuration(data.system.uptime, 'just now') : '—');
</script>

<div class="mx-auto max-w-3xl space-y-6">
	{#if showRestartDialog}
		<RestartConfirmDialog oncancel={() => (showRestartDialog = false)} />
	{/if}

	<AboutModal
		open={showAbout}
		current={version}
		platform={data.update?.platform ?? '—'}
		lastCheck={data.update?.last_check}
		onclose={() => (showAbout = false)}
	/>

	<header class="space-y-1">
		<h1 class="h2" data-testid="dashboard-heading">Dashboard</h1>
		<p class="text-surface-500-400 text-sm">A glance at what your frame is doing right now.</p>
	</header>

	<!-- Surfaces itself only when an update is available or was just rolled back -->
	<UpdatePanel status={data.update ?? null} />

	<!-- Photo beside tiles + screen on desktop, stacked on mobile -->
	<div class="grid gap-3 md:grid-cols-2">
		<NowPlaying
			image={sse.image}
			interval={data.config?.slideshow.interval ?? ''}
			shuffle={data.config?.slideshow.randomize ?? false}
		/>

		<div class="flex flex-col gap-3 md:h-full">
			<div class="reveal order-2 grid grid-cols-2 gap-3 delay-75 md:order-1">
				<StatTile
					label="WiFi"
					value={wifi.value}
					sub={wifi.sub}
					Icon={data.wifi ? WifiIcon : WifiOffIcon}
					tone={data.wifi ? 'success' : 'surface'}
					href={resolve('/admin/wifi')}
					data-testid="tile-wifi"
				/>
				<StatTile
					label="Weather"
					value={weather.value}
					sub={weather.sub}
					Icon={CloudSunIcon}
					data-testid="tile-weather"
				/>
				<StatTile
					label="Sensors"
					value={String(sensorCount)}
					sub={sensorSub(sensorCount, staleCount)}
					Icon={RadioIcon}
					tone={staleCount > 0 ? 'warning' : 'primary'}
					data-testid="tile-sensors"
				/>
				<StatTile
					label="Library"
					value={libraryValue}
					sub={librarySyncIssue ? 'Sync issue' : undefined}
					Icon={librarySyncIssue ? CloudAlertIcon : ImagesIcon}
					tone={librarySyncIssue ? 'warning' : 'primary'}
					href={resolve('/admin/images')}
					data-testid="tile-library"
				/>
			</div>

			<div
				class="card bg-surface-100-900 reveal order-1 flex flex-1 flex-col p-6 delay-150 md:order-2"
			>
				{#if data.screen === null && sse.screen === null}
					<p class="text-error-500">Could not load screen state: {data.screenError}</p>
				{:else}
					<div class="flex flex-1 items-center justify-between gap-4">
						<div class="flex items-center gap-3">
							<MonitorIcon
								class="{screenLiveOn ? 'text-success-500' : 'text-surface-500'} size-7 shrink-0"
							/>
							<div>
								<p class="font-medium" data-testid="screen-status">
									Screen {screenLiveOn ? 'on' : 'off'}
								</p>
								<p class="text-surface-500-400 text-sm">{screenReason(screenAuto, screenLiveOn)}</p>
							</div>
						</div>
						<Switch
							checked={screenAuto}
							disabled={toggleScreenBusy}
							onCheckedChange={({ checked }) => handleToggle(checked)}
							data-testid="screen-switch"
						>
							<Switch.HiddenInput />
							<Switch.Control><Switch.Thumb /></Switch.Control>
							<Switch.Label>{screenAuto ? 'Auto' : 'Off'}</Switch.Label>
						</Switch>
					</div>
				{/if}
			</div>
		</div>
	</div>

	<div class="reveal space-y-3 delay-200">
		<div class="flex items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<RadioIcon class="text-primary-500 size-5" />
				<h2 class="h4">Sensor readings</h2>
			</div>
			{#if staleCount > 0}
				<span class="badge preset-tonal-error text-xs">{staleCount} stale</span>
			{/if}
		</div>
		{#if sensorCount === 0}
			<p class="text-surface-500-400">No readings yet. They appear here as sensors report.</p>
		{:else}
			<SensorReadings sensors={sse.sensors} {now} />
		{/if}
	</div>

	<div class="card bg-surface-100-900 reveal space-y-4 p-6 delay-300">
		<div class="flex items-center justify-between gap-2">
			<div class="flex items-center gap-2">
				<ServerIcon class="text-primary-500 size-5" />
				<h2 class="h4">System</h2>
			</div>
			<button
				class="btn btn-sm preset-tonal-surface flex items-center gap-1.5"
				data-testid="dashboard-restart"
				onclick={() => (showRestartDialog = true)}
			>
				<RotateCcwIcon class="text-error-500 size-4" />
				Restart
			</button>
		</div>

		<dl class="grid grid-cols-2 gap-x-4 gap-y-3 text-sm sm:grid-cols-4">
			<div>
				<dt class="text-surface-500-400 text-xs">Version</dt>
				<dd>
					<button
						type="button"
						title="About this frame"
						class="group hover:text-primary-500 inline-flex max-w-full items-center gap-1 font-medium"
						onclick={() => (showAbout = true)}
						data-testid="dashboard-version"
					>
						<span class="truncate">{version}</span>
						<InfoIcon class="text-surface-400-500 group-hover:text-primary-500 size-3.5 shrink-0" />
					</button>
				</dd>
			</div>
			<div>
				<dt class="text-surface-500-400 text-xs">Uptime</dt>
				<dd class="truncate font-medium" data-testid="system-uptime">{uptime}</dd>
			</div>
			<div>
				<dt class="text-surface-500-400 text-xs">Hostname</dt>
				<dd class="truncate font-medium" data-testid="system-hostname">
					{data.system?.hostname || '—'}
				</dd>
			</div>
			<div>
				<dt class="text-surface-500-400 text-xs">IP address</dt>
				<dd class="truncate font-medium">{data.system?.ip || '—'}</dd>
			</div>
		</dl>

		{#if restartPending}
			<p class="text-warning-600-400 text-sm">Restarting applies your pending settings changes.</p>
		{/if}
	</div>
</div>
