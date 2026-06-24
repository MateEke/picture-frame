<script lang="ts">
	import { getSSEContext } from '$lib/sse.svelte';
	import {
		formatClockParts,
		formatMonthDay,
		formatWeekday,
		isSensorStale,
		resolveOutsideTemp
	} from '$lib/helpers';
	import { onMount } from 'svelte';
	import { weatherIconFor } from './weather-icons';

	const sse = getSSEContext();

	const DEFAULT_LOCALE = 'en-US';
	let locale = $derived(sse.kiosk?.locale || DEFAULT_LOCALE);
	let timeZone = $derived(sse.kiosk?.timezone ?? '');
	let hideClockDate = $derived(sse.kiosk?.hide_clock_date ?? false);
	let configuredSensors = $derived(new Set(sse.kiosk?.sensors ?? []));
	let weatherEnabled = $derived(sse.kiosk?.weather ?? false);
	let labels = $derived(sse.kiosk?.labels ?? { outside: '', inside: '', humidity: '' });
	let showInsideTemp = $derived(configuredSensors.has('inside:temperature'));
	let showHumidity = $derived(configuredSensors.has('inside:humidity'));
	let showOutside = $derived(configuredSensors.has('outside:temperature') || weatherEnabled);
	let showReadings = $derived(showOutside || showInsideTemp || showHumidity);
	let showOverlay = $derived(!hideClockDate || showReadings);

	let now = $state(new Date());
	let clock = $derived(formatClockParts(now, locale, timeZone));
	let weekday = $derived(formatWeekday(now, locale, timeZone));
	let monthDay = $derived(formatMonthDay(now, locale, timeZone));

	onMount(() => {
		let timeout: ReturnType<typeof setTimeout>;

		const tick = () => {
			now = new Date();
			const msToNextMinute = 60_000 - (now.getSeconds() * 1000 + now.getMilliseconds());
			timeout = setTimeout(tick, msToNextMinute);
		};
		tick();

		return () => clearTimeout(timeout);
	});

	let insideTemp = $derived.by(() => {
		const reading = sse.sensors['inside:temperature'];
		return reading && !isSensorStale(reading.timestamp) ? reading.value.toFixed(1) : '--';
	});
	let humidity = $derived.by(() => {
		const reading = sse.sensors['inside:humidity'];
		return reading && !isSensorStale(reading.timestamp) ? reading.value.toFixed(0) : '--';
	});

	let outsideTemp = $derived(resolveOutsideTemp(sse.sensors, sse.weather));

	let weatherIcon = $derived(weatherIconFor(sse.weather?.icon_code ?? '01d'));
</script>

{#if showOverlay}
	<div
		data-testid="kiosk-overlay"
		class="fixed inset-x-0 bottom-0 flex items-end justify-between bg-linear-to-b from-transparent to-black/30 px-15 pt-33 pb-12 text-kiosk-fg text-shadow-lg/30 portrait:flex-col portrait:items-start portrait:gap-12"
	>
		{#if !hideClockDate}
			<div>
				<div
					data-testid="kiosk-clock"
					class="flex items-baseline text-[7rem] leading-none font-medium tabular-nums"
				>
					<span>{clock.hours}</span>
					<span class="-mx-1 normal-nums">{clock.separator}</span>
					<span>{clock.minutes}</span>
					{#if clock.period}
						<span
							class={[
								'text-5xl font-semibold tracking-widest',
								clock.periodFirst ? 'order-first mr-2' : 'ml-2'
							]}>{clock.period}</span
						>
					{/if}
				</div>
				<div class="mt-5 mb-4.5 h-0.5 w-16 bg-kiosk-fg/80 shadow-xs/50"></div>
				<div
					data-testid="kiosk-date"
					class="text-3xl font-semibold tracking-widest uppercase opacity-94"
				>
					{weekday} <span class="opacity-60">·</span>
					{monthDay}
				</div>
			</div>
		{/if}

		{#if showReadings}
			<div class="flex gap-9 text-right portrait:text-left ml-auto portrait:ml-0">
				{#if showOutside}
					<div class="min-w-60 portrait:min-w-0">
						<div
							class="flex items-baseline justify-end gap-4 text-5xl font-normal portrait:justify-start"
						>
							{#if weatherEnabled}
								<img
									src={weatherIcon}
									alt=""
									data-testid="kiosk-weather-icon"
									class="-my-5 h-22 w-22 self-center drop-shadow-lg/60"
								/>
							{/if}
							<span data-testid="kiosk-temp-outside">{outsideTemp}°</span>
						</div>
						{#if labels.outside}
							<!-- Portrait: offset past the icon (w-22 + gap-4) so the label sits under the temp. -->
							<div
								data-testid="kiosk-label-outside"
								class="cluster-label {weatherEnabled ? 'portrait:ml-26' : ''}"
							>
								{labels.outside}
							</div>
						{/if}
					</div>
				{/if}
				{#if showInsideTemp}
					<div class="min-w-32 portrait:min-w-0">
						<div data-testid="kiosk-temp-inside" class="text-5xl font-normal">{insideTemp}°</div>
						{#if labels.inside}
							<div data-testid="kiosk-label-inside" class="cluster-label">
								{labels.inside}
							</div>
						{/if}
					</div>
				{/if}
				{#if showHumidity}
					<div class="min-w-30 portrait:min-w-0">
						<div class="text-5xl font-normal">{humidity}%</div>
						{#if labels.humidity}
							<div data-testid="kiosk-label-humidity" class="cluster-label">
								{labels.humidity}
							</div>
						{/if}
					</div>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<style lang="postcss">
	@reference "tailwindcss";
	.cluster-label {
		@apply mt-1.5 text-sm font-medium tracking-wider uppercase opacity-80;
	}
</style>
