<script lang="ts">
	import type { Component } from 'svelte';
	import type { SensorPayload } from '$lib/api/types.gen';
	import { ThermometerIcon, DropletIcon, RadarIcon, GaugeIcon } from '@lucide/svelte';
	import { formatSensorValue, timeAgo, isSensorStale } from '$lib/helpers';

	let { sensors, now }: { sensors: Record<string, SensorPayload>; now: number } = $props();

	const KIND_ICONS: Record<string, Component> = {
		temperature: ThermometerIcon,
		humidity: DropletIcon,
		motion: RadarIcon
	};
	function kindIcon(kind: string): Component {
		return KIND_ICONS[kind] ?? GaugeIcon;
	}

	// Motion is on/off: amber when active, green when clear; others stay primary.
	function iconTone(sensor: SensorPayload): string {
		if (sensor.kind !== 'motion') return 'text-primary-500';
		return sensor.value !== 0 ? 'text-warning-500' : 'text-success-500';
	}

	const rows = $derived(
		Object.entries(sensors)
			.map(([key, sensor]) => ({ key, sensor }))
			.sort(
				(a, b) =>
					(a.sensor.role || a.sensor.device_id).localeCompare(
						b.sensor.role || b.sensor.device_id
					) || a.sensor.kind.localeCompare(b.sensor.kind)
			)
	);
</script>

<!-- auto-fit makes columns track the count (1 fills the row, 2 split it…); 10rem min caps it at 4 -->
<div class="grid grid-cols-[repeat(auto-fit,minmax(min(100%,10rem),1fr))] gap-2">
	{#each rows as { key, sensor } (key)}
		{const Icon = kindIcon(sensor.kind)}
		{const stale = isSensorStale(sensor.timestamp)}
		<div
			data-testid="sensor-reading"
			class="card bg-surface-100-900 reveal flex flex-col gap-1 p-3 {stale ? 'opacity-60' : ''}"
		>
			<div class="flex items-center gap-2">
				<Icon class="size-5 shrink-0 {iconTone(sensor)}" />
				<p class="truncate font-semibold">{formatSensorValue(sensor.kind, sensor.value)}</p>
			</div>
			<p class="text-surface-500-400 truncate text-xs">
				<span class="capitalize">{sensor.role || sensor.device_id}</span>
				· {sensor.timestamp ? timeAgo(sensor.timestamp, now) : '—'}
				{#if stale}<span class="text-error-500">· stale</span>{/if}
			</p>
		</div>
	{/each}
</div>
