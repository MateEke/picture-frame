<script lang="ts">
	import type { SensorDto, ConfigMetaBody } from '$lib/api/types.gen';
	import { PencilIcon, PlusIcon, TrashIcon } from '@lucide/svelte';
	import SensorDialog from './SensorDialog.svelte';
	import Field from './Field.svelte';
	import ToggleRow from './ToggleRow.svelte';
	import LanFields from './LanFields.svelte';
	import DeviceCombobox from './DeviceCombobox.svelte';

	const TYPE_LABELS: Record<string, string> = {
		ble: 'Bluetooth',
		'mqtt-subscriber': 'MQTT',
		mock: 'Mock',
		lan: 'WiFi presence'
	};

	function sensorKinds(s: SensorDto): string[] {
		const kinds: string[] = [];
		const add = (k?: string) => {
			if (k && !kinds.includes(k)) kinds.push(k);
		};
		for (const c of s.characteristics ?? []) add(c.kind);
		for (const r of s.mock_readings ?? []) add(r.kind);
		add(s.kind);
		// LAN presence emits motion=1 as its display keepalive.
		if (s.type === 'lan') add('motion');
		return kinds;
	}

	let {
		sensors = $bindable(),
		savedSensors,
		bluetoothAdapter = $bindable(),
		savedBluetoothAdapter,
		adapters,
		meta,
		errors
	}: {
		sensors: SensorDto[] | null;
		savedSensors: SensorDto[] | null;
		bluetoothAdapter: string;
		savedBluetoothAdapter: string;
		adapters: string[];
		meta: ConfigMetaBody;
		errors?: Record<string, string>;
	} = $props();

	// undefined = closed, null = adding new, SensorDto = editing existing
	let dialogSensor = $state<SensorDto | null | undefined>(undefined);

	// Phone presence manages the LAN sensor(s) directly: toggle + hosts inline.
	const lanOf = (list: SensorDto[] | null | undefined) =>
		(list ?? []).filter((s) => s.type === 'lan');
	const lanIdx = $derived((sensors ?? []).findIndex((s) => s.type === 'lan'));
	const presenceOn = $derived(lanIdx >= 0);
	const presenceChanged = $derived(
		JSON.stringify(lanOf(sensors)) !== JSON.stringify(lanOf(savedSensors))
	);

	function setPresence(on: boolean) {
		if (on && lanIdx < 0) {
			const taken = new Set((sensors ?? []).map((s) => s.id));
			let id = 'phones';
			for (let n = 2; taken.has(id); n++) id = `phones_${n}`;
			sensors = [
				...(sensors ?? []),
				{ id, type: 'lan', role: 'presence', hosts: [], poll_interval: '30s' }
			];
		} else if (!on) {
			sensors = (sensors ?? []).filter((s) => s.type !== 'lan');
		}
	}

	function revertPresence() {
		sensors = [
			...(sensors ?? []).filter((s) => s.type !== 'lan'),
			...structuredClone(lanOf(savedSensors))
		];
	}

	function handleSensorSave(newSensor: SensorDto, oldSensor?: SensorDto | null) {
		const list = sensors ?? [];
		const idx = oldSensor ? list.findIndex((s) => s.id === oldSensor.id) : -1;
		if (idx >= 0) {
			sensors![idx] = newSensor;
		} else {
			sensors = [...list, newSensor];
		}
		dialogSensor = undefined;
	}

	function removeSensor(id: string) {
		sensors = (sensors ?? []).filter((s) => s.id !== id);
	}
</script>

<div class="space-y-4">
	<Field
		label="Bluetooth adapter"
		help="HCI device used for BLE sensors. Pick a detected adapter or type one."
		changed={bluetoothAdapter !== savedBluetoothAdapter}
		onrevert={() => (bluetoothAdapter = savedBluetoothAdapter)}
	>
		<DeviceCombobox bind:value={bluetoothAdapter} options={adapters} placeholder="hci0" />
	</Field>
	<div class="border-surface-300-700 space-y-4 border-t pt-4">
		<ToggleRow
			label="Phone presence (WiFi ping)"
			checked={presenceOn}
			changed={presenceChanged}
			onchange={setPresence}
			onrevert={revertPresence}
			testId="presence-toggle"
		/>
		<p class="text-surface-500-400 -mt-2 text-xs">
			Ping your phones' static IPs; any phone home keeps the screen on. The idle-blank timeout is
			the away grace.
		</p>
		{#if sensors && lanIdx >= 0}
			<LanFields bind:draft={sensors[lanIdx]} testIdPrefix="presence" />
		{/if}
	</div>
	<div class="border-surface-300-700 space-y-2 border-t pt-4">
		<span class="label-text">Sensors</span>
		{#if (sensors ?? []).length === 0}
			<p class="text-surface-500-400 py-2 text-sm">No sensors yet.</p>
		{/if}
		{#each sensors ?? [] as sensor (sensor.id)}
			{const sensorError = $derived(errors?.[sensor.id])}
			<div
				data-testid="sensor-row-{sensor.id}"
				class="rounded-lg border px-3 py-2 {sensorError
					? 'border-error-500'
					: 'border-surface-300-700'}"
			>
				<div class="flex items-center justify-between gap-2">
					<div class="min-w-0 space-y-1">
						<div class="flex items-center gap-2">
							<span class="truncate font-medium">{sensor.role || sensor.id}</span>
							<span class="badge preset-tonal-surface text-xs"
								>{TYPE_LABELS[sensor.type] ?? sensor.type}</span
							>
						</div>
						<div class="text-surface-500-400 flex flex-wrap items-center gap-1.5 text-xs">
							<span class="font-mono">{sensor.id}</span>
							{#each sensorKinds(sensor) as kind (kind)}
								<span class="badge preset-tonal-primary text-xs">{kind}</span>
							{/each}
						</div>
					</div>
					<div class="flex shrink-0 gap-1">
						<button
							class="btn-icon btn-icon-sm preset-tonal-surface"
							aria-label="Edit {sensor.id}"
							data-testid="sensor-edit-{sensor.id}"
							onclick={() => (dialogSensor = sensor)}
						>
							<PencilIcon size={14} />
						</button>
						<button
							class="btn-icon btn-icon-sm preset-tonal-error"
							aria-label="Delete {sensor.id}"
							data-testid="sensor-delete-{sensor.id}"
							onclick={() => removeSensor(sensor.id)}
						>
							<TrashIcon size={14} />
						</button>
					</div>
				</div>
				{#if sensorError}
					<p class="text-error-500 mt-1.5 text-xs">{sensorError}</p>
				{/if}
			</div>
		{/each}
		<button
			class="btn preset-tonal-primary w-full"
			data-testid="sensor-add"
			onclick={() => (dialogSensor = null)}
		>
			<PlusIcon size={16} /> Add sensor
		</button>
	</div>
</div>

{#if dialogSensor !== undefined}
	<SensorDialog
		sensor={dialogSensor}
		existing={sensors ?? []}
		{meta}
		onSave={handleSensorSave}
		onCancel={() => (dialogSensor = undefined)}
	/>
{/if}
