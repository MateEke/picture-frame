<script lang="ts">
	import type { WeatherDto } from '$lib/api/types.gen';
	import { DURATION_STOPS } from '$lib/duration';
	import SecretField from '$lib/SecretField.svelte';
	import DurationSlider from './DurationSlider.svelte';
	import Field from './Field.svelte';

	let {
		weather = $bindable(),
		savedWeather,
		units,
		errors
	}: {
		weather: WeatherDto;
		savedWeather: WeatherDto;
		units: string[] | null;
		errors?: { lat?: string; lon?: string };
	} = $props();
</script>

<div class="space-y-4">
	<SecretField
		label="OpenWeatherMap API key"
		warningText="API key will be removed on save."
		bind:value={weather.api_key}
		bind:isSet={weather.api_key_set}
		wasSet={savedWeather.api_key_set}
	/>
	<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
		<Field
			label="Latitude"
			error={errors?.lat}
			changed={weather.lat !== savedWeather.lat}
			onrevert={() => (weather.lat = savedWeather.lat)}
		>
			<input
				class="input"
				type="number"
				step="any"
				bind:value={weather.lat}
				data-testid="setting-weather-lat"
			/>
		</Field>
		<Field
			label="Longitude"
			error={errors?.lon}
			changed={weather.lon !== savedWeather.lon}
			onrevert={() => (weather.lon = savedWeather.lon)}
		>
			<input class="input" type="number" step="any" bind:value={weather.lon} />
		</Field>
		<DurationSlider
			label="Poll interval"
			stops={DURATION_STOPS.weatherPoll}
			bind:value={weather.poll_interval}
			changed={weather.poll_interval !== savedWeather.poll_interval}
			onrevert={() => (weather.poll_interval = savedWeather.poll_interval)}
		/>
		<DurationSlider
			label="Retry interval"
			help="First delay after a failed poll. Backs off up to the poll interval."
			stops={DURATION_STOPS.weatherRetry}
			bind:value={weather.retry_interval}
			changed={weather.retry_interval !== savedWeather.retry_interval}
			onrevert={() => (weather.retry_interval = savedWeather.retry_interval)}
		/>
		<Field
			label="Units"
			changed={weather.units !== savedWeather.units}
			onrevert={() => (weather.units = savedWeather.units)}
		>
			<select class="select" bind:value={weather.units}>
				{#each units ?? ['standard', 'metric', 'imperial'] as u (u)}
					<option value={u}>{u}</option>
				{/each}
			</select>
		</Field>
	</div>
</div>
