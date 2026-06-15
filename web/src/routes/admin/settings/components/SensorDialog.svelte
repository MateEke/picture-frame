<script lang="ts">
	import type { SensorDto, ConfigMetaBody } from '$lib/api/types.gen';
	import { untrack } from 'svelte';
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
	import { RadioIcon } from '@lucide/svelte';
	import Field from './Field.svelte';
	import BleFields from './BleFields.svelte';
	import MqttSubscriberFields from './MqttSubscriberFields.svelte';
	import MockFields from './MockFields.svelte';
	import { sensorTypeError } from '../validate';

	let {
		sensor,
		existing,
		meta,
		onSave,
		onCancel
	}: {
		sensor: SensorDto | null;
		existing: SensorDto[];
		meta: ConfigMetaBody;
		onSave: (newSensor: SensorDto, oldSensor?: SensorDto | null) => void;
		onCancel: () => void;
	} = $props();

	const isNew = $derived(sensor === null);

	const TYPE_LABELS: Record<string, string> = {
		ble: 'Bluetooth',
		'mqtt-subscriber': 'MQTT',
		mock: 'Mock'
	};

	let draft = $state<SensorDto>(createEmpty());

	$effect.pre(() => {
		const snap = $state.snapshot(sensor);
		const initial = snap ? structuredClone(snap) : createEmpty();
		untrack(() => {
			draft = initial;
		});
	});

	function createEmpty(): SensorDto {
		return { id: '', type: 'mock', role: '', characteristics: [], mock_readings: [] };
	}

	function toSensorType(v: string | null): SensorDto['type'] | undefined {
		if (v === 'ble' || v === 'mqtt-subscriber' || v === 'mock') return v;
		return undefined;
	}

	function collectKinds(s: SensorDto): string[] {
		const kinds: string[] = [];
		for (const c of s.characteristics ?? []) if (c.kind) kinds.push(c.kind);
		for (const r of s.mock_readings ?? []) if (r.kind) kinds.push(r.kind);
		if (s.kind) kinds.push(s.kind);
		return kinds;
	}

	const idConflict = $derived.by(() => {
		if (!draft.id) return undefined;
		const others = existing.filter((s) => s.id !== sensor?.id);
		return others.some((s) => s.id === draft.id) ? `ID "${draft.id}" is already in use` : undefined;
	});

	const roleKindConflict = $derived.by(() => {
		const others = existing.filter((s) => s.id !== sensor?.id);
		const myKinds = collectKinds(draft);
		for (const other of others) {
			if (!draft.role || other.role !== draft.role) continue;
			for (const k of myKinds) {
				if (collectKinds(other).includes(k))
					return `Role "${draft.role}" already has a "${k}" reading from sensor "${other.id}"`;
			}
		}
		return null;
	});

	// Rejects what the backend would: ble mac/characteristics, mqtt topic/kind/parser.
	const typeError = $derived(sensorTypeError(draft));

	const canSave = $derived(
		draft.id.trim() !== '' &&
			draft.role.trim() !== '' &&
			!idConflict &&
			roleKindConflict === null &&
			typeError === null
	);

	function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!canSave) return;
		onSave(draft, sensor);
	}
</script>

<Dialog
	open
	onOpenChange={(d: { open: boolean }) => {
		if (!d.open) onCancel();
	}}
>
	<Portal>
		<Dialog.Backdrop class="fixed inset-0 z-50 bg-black/50" />
		<Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
			<Dialog.Content
				class="card bg-surface-100-900 max-h-[90vh] w-full max-w-lg overflow-y-auto p-6 shadow-xl"
			>
				<Dialog.Title class="h4 flex items-center gap-2">
					<RadioIcon class="text-primary-500 size-5" />
					{isNew ? 'Add sensor' : 'Edit sensor'}
				</Dialog.Title>
				<form class="mt-5 space-y-5" onsubmit={handleSubmit}>
					<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
						<Field label="ID" help="Unique name for this sensor." error={idConflict}>
							<input
								class="input"
								type="text"
								bind:value={draft.id}
								placeholder="sensor1"
								required
								data-testid="sensor-dialog-id"
							/>
						</Field>
						<Field label="Role" help="Groups readings on the frame, e.g. inside or outside.">
							<input
								class="input"
								type="text"
								bind:value={draft.role}
								placeholder="inside"
								required
								data-testid="sensor-dialog-role"
							/>
						</Field>
					</div>

					<Field label="Type" help="Bluetooth (BLE), an MQTT topic, or a built-in mock source.">
						<select
							class="select"
							value={draft.type}
							onchange={(e) => (draft.type = toSensorType(e.currentTarget.value) ?? draft.type)}
						>
							{#each meta.sensor_types ?? [] as t (t)}
								<option value={t}>{TYPE_LABELS[t] ?? t}</option>
							{/each}
						</select>
					</Field>

					{#if draft.type === 'ble'}
						<BleFields bind:draft {meta} />
					{:else if draft.type === 'mqtt-subscriber'}
						<MqttSubscriberFields bind:draft {meta} />
					{:else if draft.type === 'mock'}
						<MockFields bind:draft {meta} />
					{/if}

					{#if typeError}
						<p class="text-error-500 text-sm">{typeError}</p>
					{/if}
					{#if roleKindConflict}
						<p class="text-error-500 text-sm">{roleKindConflict}</p>
					{/if}

					<div class="flex justify-end gap-2 pt-2">
						<button
							type="button"
							class="btn preset-tonal-surface"
							onclick={onCancel}
							data-testid="sensor-dialog-cancel">Cancel</button
						>
						<button
							type="submit"
							class="btn preset-tonal-primary"
							disabled={!canSave}
							data-testid="sensor-dialog-save"
						>
							{isNew ? 'Add' : 'Save'}
						</button>
					</div>
				</form>
			</Dialog.Content>
		</Dialog.Positioner>
	</Portal>
</Dialog>
