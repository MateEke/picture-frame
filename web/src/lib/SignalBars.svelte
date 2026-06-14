<script lang="ts">
	import { signalLevel } from './wifi';

	let { signal }: { signal: number } = $props();

	const level = $derived(signalLevel(signal));
	// Four bars of increasing height; index < level renders filled.
	const bars = [{ h: 'h-1.5' }, { h: 'h-2.5' }, { h: 'h-3.5' }, { h: 'h-4.5' }];
</script>

<span
	class="flex h-4.5 items-end gap-0.5"
	role="img"
	aria-label="Signal strength {signal}%"
	title="{signal}%"
>
	{#each bars as bar, i (i)}
		<span class="w-1 rounded-xs {bar.h} {i < level ? 'bg-surface-700-300' : 'bg-surface-300-700'}"
		></span>
	{/each}
</span>
