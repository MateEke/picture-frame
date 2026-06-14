<script lang="ts">
	import type { ConfigDto } from '$lib/api/types.gen';
	import Field from './Field.svelte';

	let {
		logLevel = $bindable(),
		savedLogLevel,
		levels
	}: {
		logLevel: ConfigDto['log_level'];
		savedLogLevel: ConfigDto['log_level'];
		levels: string[];
	} = $props();
</script>

<Field
	label="Log level"
	help="Verbosity of the frame's logs. Applies immediately, no restart needed."
	changed={logLevel !== savedLogLevel}
	onrevert={() => (logLevel = savedLogLevel)}
>
	<select class="select" bind:value={logLevel}>
		{#each levels as level (level)}
			<option value={level}>{level}</option>
		{/each}
	</select>
</Field>
