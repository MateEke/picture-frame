<script lang="ts">
	import type { SensorDto, ConfigMetaBody } from '$lib/api/types.gen';
	import Field from './Field.svelte';

	let {
		draft = $bindable(),
		meta
	}: {
		draft: SensorDto;
		meta: ConfigMetaBody;
	} = $props();
</script>

<Field label="Topic" help="MQTT topic this sensor's readings are published on.">
	<input class="input" type="text" bind:value={draft.topic} placeholder="sensors/room/temp" />
</Field>
<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
	<Field label="Kind">
		<select class="select" bind:value={draft.kind}>
			<option value="">—</option>
			{#each meta.kinds ?? [] as k (k)}
				<option value={k}>{k}</option>
			{/each}
		</select>
	</Field>
	<Field label="Parser" help="How to decode the payload into a value.">
		<select class="select" bind:value={draft.parser}>
			<option value="">—</option>
			{#each meta.decoders ?? [] as d (d)}
				<option value={d}>{d}</option>
			{/each}
		</select>
	</Field>
</div>
<Field
	label="JSON field"
	help="Extract a value by dotted path, e.g. main.temp. Leave blank for raw payload."
>
	<input class="input" type="text" bind:value={draft.json_field} placeholder="main.temp" />
</Field>
