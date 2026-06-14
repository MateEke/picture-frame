<script lang="ts">
	import type { MqttDto } from '$lib/api/types.gen';
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import { DURATION_STOPS } from '$lib/duration';
	import SecretField from '$lib/SecretField.svelte';
	import DurationSlider from './DurationSlider.svelte';
	import Field from './Field.svelte';

	let {
		mqtt = $bindable(),
		savedMqtt,
		errors
	}: {
		mqtt: MqttDto;
		savedMqtt: MqttDto;
		errors?: { broker?: string; node_id?: string; base_topic?: string; discovery_prefix?: string };
	} = $props();

	// The bridge needs a broker to connect, so it can't be enabled until one is set.
	const brokerSet = $derived(mqtt.broker.trim().length > 0);
</script>

<div class="space-y-4">
	<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
		<Field
			label="Broker"
			error={errors?.broker}
			changed={mqtt.broker !== savedMqtt.broker}
			onrevert={() => (mqtt.broker = savedMqtt.broker)}
		>
			<input
				class="input"
				type="text"
				bind:value={mqtt.broker}
				placeholder="tcp://192.168.1.10:1883"
				data-testid="mqtt-broker"
			/>
		</Field>
		<Field
			label="Client ID"
			help="Identifies this frame to the broker. Keep it unique."
			changed={mqtt.client_id !== savedMqtt.client_id}
			onrevert={() => (mqtt.client_id = savedMqtt.client_id)}
		>
			<input class="input" type="text" bind:value={mqtt.client_id} placeholder="picture-frame" />
		</Field>
		<Field
			label="Username"
			changed={mqtt.username !== savedMqtt.username}
			onrevert={() => (mqtt.username = savedMqtt.username)}
		>
			<input class="input" type="text" bind:value={mqtt.username} />
		</Field>
		<SecretField
			label="Password"
			warningText="Password will be removed on save."
			bind:value={mqtt.password}
			bind:isSet={mqtt.password_set}
			wasSet={savedMqtt.password_set}
		/>
	</div>

	<div class="border-surface-300-700 space-y-4 border-t pt-4">
		<div class="flex items-center gap-3">
			<Switch
				checked={mqtt.bridge.enabled}
				disabled={!brokerSet}
				onCheckedChange={({ checked }) => (mqtt.bridge.enabled = checked)}
				data-testid="mqtt-bridge-switch"
			>
				<Switch.HiddenInput />
				<Switch.Control><Switch.Thumb /></Switch.Control>
				<Switch.Label>Home Assistant bridge</Switch.Label>
			</Switch>
			{#if !brokerSet}
				<span class="text-surface-500-400 text-xs" data-testid="mqtt-broker-hint">
					Set a broker to enable
				</span>
			{/if}
		</div>
		{#if mqtt.bridge.enabled}
			<div class="border-surface-300-700 grid grid-cols-1 gap-4 border-l-2 pl-4 md:grid-cols-2">
				<Field
					label="Node ID"
					help="HA device id and unique-id prefix for this frame."
					error={errors?.node_id}
					changed={mqtt.bridge.node_id !== savedMqtt.bridge.node_id}
					onrevert={() => (mqtt.bridge.node_id = savedMqtt.bridge.node_id)}
				>
					<input
						class="input"
						type="text"
						bind:value={mqtt.bridge.node_id}
						placeholder="picture_frame"
						data-testid="mqtt-node-id"
					/>
				</Field>
				<Field
					label="Base topic"
					help="Namespace for state, availability, and command topics."
					error={errors?.base_topic}
					changed={mqtt.bridge.base_topic !== savedMqtt.bridge.base_topic}
					onrevert={() => (mqtt.bridge.base_topic = savedMqtt.bridge.base_topic)}
				>
					<input
						class="input"
						type="text"
						bind:value={mqtt.bridge.base_topic}
						placeholder="picture-frame"
					/>
				</Field>
				<Field
					label="Discovery prefix"
					help="Home Assistant discovery prefix (config topics)."
					error={errors?.discovery_prefix}
					changed={mqtt.bridge.discovery_prefix !== savedMqtt.bridge.discovery_prefix}
					onrevert={() => (mqtt.bridge.discovery_prefix = savedMqtt.bridge.discovery_prefix)}
				>
					<input
						class="input"
						type="text"
						bind:value={mqtt.bridge.discovery_prefix}
						placeholder="homeassistant"
					/>
				</Field>
				<DurationSlider
					label="Stale after"
					help="Report a sensor offline in HA if no reading arrives within this."
					stops={DURATION_STOPS.mqttStale}
					bind:value={mqtt.bridge.stale_after}
					changed={mqtt.bridge.stale_after !== savedMqtt.bridge.stale_after}
					onrevert={() => (mqtt.bridge.stale_after = savedMqtt.bridge.stale_after)}
				/>
			</div>
		{/if}
	</div>
</div>
