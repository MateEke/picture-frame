<script lang="ts">
	import type { SensorDto, CharacteristicDto, ConfigMetaBody } from '$lib/api/types.gen';
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
	if (!draft.reset_after) {
		draft.reset_after = '';
	}

	function addCharacteristic() {
		draft.characteristics = [...(draft.characteristics ?? []), { uuid: '', kind: '', decoder: '' }];
	}

	function removeCharacteristic(i: number) {
		draft.characteristics?.splice(i, 1);
	}

	function updateCharacteristic(i: number, patch: Partial<CharacteristicDto>) {
		const current = draft.characteristics?.[i];
		if (current) draft.characteristics![i] = { ...current, ...patch };
	}
</script>

<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
	<label class="label">
		<span class="label-text">MAC address</span>
		<input class="input" type="text" bind:value={draft.mac} placeholder="AA:BB:CC:DD:EE:FF" />
	</label>
	<label class="label">
		<span class="label-text">Address type</span>
		<select class="select" bind:value={draft.address_type}>
			{#each meta.address_types ?? [] as at (at)}
				<option value={at}>{at}</option>
			{/each}
		</select>
	</label>
	<DurationSlider
		label="Poll interval"
		stops={DURATION_STOPS.sensorPoll}
		bind:value={draft.poll_interval}
	/>
	<DurationSlider
		label="Reset after"
		help="Power-cycle the adapter after this long without a reading (Off disables it)."
		stops={DURATION_STOPS.sensorReset}
		zeroLabel="Off"
		bind:value={draft.reset_after}
	/>
</div>

<div class="space-y-2">
	<div class="flex items-center justify-between">
		<span class="label-text font-medium">Characteristics</span>
		<button type="button" class="btn btn-sm preset-tonal-primary" onclick={addCharacteristic}>
			<PlusIcon size={14} /> Add
		</button>
	</div>
	{#each draft.characteristics ?? [] as char, i (i)}
		<div class="grid grid-cols-[1fr_1fr_1fr_auto] items-end gap-2">
			<label class="label">
				<span class="label-text text-xs">UUID</span>
				<input
					class="input input-sm"
					type="text"
					value={char.uuid}
					oninput={(e) => updateCharacteristic(i, { uuid: e.currentTarget.value })}
				/>
			</label>
			<label class="label">
				<span class="label-text text-xs">Kind</span>
				<select
					class="select select-sm"
					value={char.kind}
					onchange={(e) => updateCharacteristic(i, { kind: e.currentTarget.value })}
				>
					<option value="">—</option>
					{#each meta.kinds ?? [] as k (k)}
						<option value={k}>{k}</option>
					{/each}
				</select>
			</label>
			<label class="label">
				<span class="label-text text-xs">Decoder</span>
				<select
					class="select select-sm"
					value={char.decoder}
					onchange={(e) => updateCharacteristic(i, { decoder: e.currentTarget.value })}
				>
					<option value="">—</option>
					{#each meta.decoders ?? [] as d (d)}
						<option value={d}>{d}</option>
					{/each}
				</select>
			</label>
			<button
				type="button"
				class="btn btn-sm preset-tonal-error mb-0.5"
				onclick={() => removeCharacteristic(i)}
			>
				<XIcon size={14} />
			</button>
		</div>
	{/each}
</div>
