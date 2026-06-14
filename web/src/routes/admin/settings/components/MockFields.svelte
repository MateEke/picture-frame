<script lang="ts">
	import type { SensorDto, MockReadingDto, ConfigMetaBody } from '$lib/api/types.gen';
	import { PlusIcon, XIcon } from '@lucide/svelte';
	import { DURATION_STOPS } from '$lib/duration';
	import DurationSlider from './DurationSlider.svelte';

	let {
		draft = $bindable(),
		meta
	}: {
		draft: SensorDto;
		meta: ConfigMetaBody;
	} = $props();

	if (!draft.poll_interval) {
		draft.poll_interval = '';
	}

	function addReading() {
		draft.mock_readings = [...(draft.mock_readings ?? []), { kind: '', value: 0, delta: 0 }];
	}

	function removeReading(i: number) {
		draft.mock_readings?.splice(i, 1);
	}

	function updateReading(i: number, patch: Partial<MockReadingDto>) {
		const current = draft.mock_readings?.[i];
		if (current) draft.mock_readings![i] = { ...current, ...patch };
	}
</script>

<div class="space-y-2">
	<div class="flex items-center justify-between">
		<span class="label-text font-medium">Readings</span>
		<button type="button" class="btn btn-sm preset-tonal-primary" onclick={addReading}>
			<PlusIcon size={14} /> Add
		</button>
	</div>
	{#each draft.mock_readings ?? [] as reading, i (i)}
		<div class="grid grid-cols-[1fr_auto_auto_auto] items-end gap-2">
			<label class="label">
				<span class="label-text text-xs">Kind</span>
				<select
					class="select select-sm"
					value={reading.kind}
					onchange={(e) => updateReading(i, { kind: e.currentTarget.value })}
				>
					<option value="">—</option>
					{#each meta.kinds ?? [] as k (k)}
						<option value={k}>{k}</option>
					{/each}
				</select>
			</label>
			<label class="label">
				<span class="label-text text-xs">Value</span>
				<input
					class="input input-sm w-20"
					type="number"
					step="any"
					value={reading.value}
					oninput={(e) => updateReading(i, { value: parseFloat(e.currentTarget.value) || 0 })}
				/>
			</label>
			<label class="label">
				<span class="label-text text-xs">Delta</span>
				<input
					class="input input-sm w-20"
					type="number"
					step="any"
					value={reading.delta}
					oninput={(e) => updateReading(i, { delta: parseFloat(e.currentTarget.value) || 0 })}
				/>
			</label>
			<button
				type="button"
				class="btn btn-sm preset-tonal-error mb-0.5"
				onclick={() => removeReading(i)}
			>
				<XIcon size={14} />
			</button>
		</div>
	{/each}
</div>
<DurationSlider
	label="Poll interval"
	stops={DURATION_STOPS.sensorPoll}
	bind:value={draft.poll_interval}
/>
