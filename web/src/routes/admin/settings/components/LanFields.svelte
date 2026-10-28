<script lang="ts">
	import type { SensorDto } from '$lib/api/types.gen';
	import { PlusIcon, XIcon } from '@lucide/svelte';
	import { DURATION_STOPS } from '$lib/duration';
	import Field from './Field.svelte';
	import DurationSlider from './DurationSlider.svelte';

	let {
		draft = $bindable(),
		testIdPrefix = 'sensor-dialog'
	}: {
		draft: SensorDto;
		testIdPrefix?: string;
	} = $props();

	function addHost() {
		draft.hosts = [...(draft.hosts ?? []), ''];
	}

	function removeHost(i: number) {
		draft.hosts = (draft.hosts ?? []).filter((_, j) => j !== i);
	}

	function setHost(i: number, value: string) {
		const hosts = [...(draft.hosts ?? [])];
		hosts[i] = value;
		draft.hosts = hosts;
	}
</script>

<Field
	label="Phone IPs"
	help="Static IPs (DHCP reservations) of phones on your WiFi. Any reachable host keeps the screen on."
>
	<div class="space-y-2">
		{#each draft.hosts ?? [] as host, i (i)}
			<div class="flex items-center gap-2">
				<input
					class="input input-sm font-mono"
					type="text"
					value={host}
					placeholder="192.168.1.50"
					aria-label="Phone IP {i + 1}"
					data-testid="{testIdPrefix}-host-{i}"
					oninput={(e) => setHost(i, e.currentTarget.value)}
				/>
				<button
					type="button"
					class="btn btn-sm preset-tonal-error shrink-0"
					aria-label="Remove phone IP {i + 1}"
					onclick={() => removeHost(i)}
				>
					<XIcon size={14} />
				</button>
			</div>
		{/each}
		<button
			type="button"
			class="btn btn-sm preset-tonal-primary w-fit"
			onclick={addHost}
			data-testid="{testIdPrefix}-host-add"
		>
			<PlusIcon size={14} /> Add phone
		</button>
	</div>
</Field>
<DurationSlider
	label="Poll interval"
	help="How often each phone is pinged."
	stops={DURATION_STOPS.sensorPoll}
	bind:value={draft.poll_interval}
/>
