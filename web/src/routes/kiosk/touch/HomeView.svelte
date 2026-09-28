<script lang="ts">
	import { MoonIcon, PlayIcon, PowerIcon } from '@lucide/svelte';
	import { onMount } from 'svelte';
	import { getSSEContext } from '$lib/sse.svelte';
	import {
		formatClockParts,
		formatMonthDay,
		formatWeekday,
		isSensorStale,
		resolveOutsideTemp
	} from '$lib/helpers';
	import { fetchFiles, fetchImages, formatSeconds, screenOff } from '$lib/touch';
	import { weatherIconFor } from '../components/weather-icons';
	import { fi } from './fi';

	let { onStart }: { onStart: () => void } = $props();

	const sse = getSSEContext();

	// The touch UI is Finnish; follow the configured locale only for the clock's
	// 12/24h and date style, defaulting to fi-FI.
	const locale = $derived(
		sse.kiosk?.locale && sse.kiosk.locale !== 'en-US' ? sse.kiosk.locale : 'fi-FI'
	);
	const timeZone = $derived(sse.kiosk?.timezone ?? '');

	let now = $state(new Date());
	onMount(() => {
		let t: ReturnType<typeof setTimeout>;
		const tick = () => {
			now = new Date();
			t = setTimeout(tick, 60_000 - (now.getSeconds() * 1000 + now.getMilliseconds()));
		};
		tick();
		return () => clearTimeout(t);
	});

	const clock = $derived(formatClockParts(now, locale, timeZone));
	const weekday = $derived(formatWeekday(now, locale, timeZone));
	const monthDay = $derived(formatMonthDay(now, locale, timeZone));

	const configured = $derived(new Set(sse.kiosk?.sensors ?? []));
	const weatherOn = $derived(sse.kiosk?.weather ?? false);
	const outside = $derived(resolveOutsideTemp(sse.sensors, sse.weather));
	const showOutside = $derived(configured.has('outside:temperature') || weatherOn);
	const inside = $derived.by(() => {
		const r = sse.sensors['inside:temperature'];
		return r && !isSensorStale(r.timestamp) ? r.value.toFixed(1) : null;
	});
	const humidity = $derived.by(() => {
		const r = sse.sensors['inside:humidity'];
		return r && !isSensorStale(r.timestamp) ? r.value.toFixed(0) : null;
	});
	const labels = $derived(sse.kiosk?.labels);

	const sleep = $derived(sse.kiosk?.sleep);

	let photoCount = $state<number | null>(null);
	let shownCount = $state<number | null>(null);
	let fileCount = $state<number | null>(null);

	// "3 kuvaa, 2 diaesityksessä · 2 tiedostoa"
	const countsLine = $derived.by(() => {
		if (photoCount === null) return '';
		let line = `${photoCount} ${fi.home.photos}`;
		if (shownCount !== null && shownCount !== photoCount) {
			line += `, ${shownCount} ${fi.home.inSlideshow}`;
		}
		if (fileCount) line += ` · ${fileCount} ${fi.home.files}`;
		return line;
	});

	$effect(() => {
		void sse.libraryRev;
		fetchImages().then((imgs) => {
			if (!imgs) return;
			photoCount = imgs.length;
			shownCount = imgs.filter((i) => i.included).length;
		});
		fetchFiles().then((f) => {
			if (f) fileCount = f.files.length;
		});
	});
</script>

<div class="flex h-full flex-col gap-8 overflow-y-auto p-8" data-testid="touch-home">
	<section>
		<div class="flex items-baseline text-[7rem] leading-none font-medium tabular-nums">
			<span>{clock.hours}</span><span class="-mx-1">{clock.separator}</span><span
				>{clock.minutes}</span
			>
			{#if clock.period}
				<span class="ml-3 text-4xl font-semibold">{clock.period}</span>
			{/if}
		</div>
		<div class="mt-4 text-3xl font-semibold tracking-wide text-neutral-300 capitalize">
			{weekday} · {monthDay}
		</div>
	</section>

	{#if showOutside || inside || humidity}
		<section class="flex flex-wrap gap-4" data-testid="touch-home-readings">
			{#if showOutside}
				<div class="flex items-center gap-4 rounded-3xl bg-neutral-900 px-6 py-5">
					{#if weatherOn}
						<img src={weatherIconFor(sse.weather?.icon_code ?? '01d')} alt="" class="size-16" />
					{/if}
					<div>
						<div class="text-5xl">{outside}°</div>
						{#if labels?.outside}<div class="text-lg text-neutral-400">{labels.outside}</div>{/if}
					</div>
				</div>
			{/if}
			{#if inside}
				<div class="rounded-3xl bg-neutral-900 px-6 py-5">
					<div class="text-5xl">{inside}°</div>
					{#if labels?.inside}<div class="text-lg text-neutral-400">{labels.inside}</div>{/if}
				</div>
			{/if}
			{#if humidity}
				<div class="rounded-3xl bg-neutral-900 px-6 py-5">
					<div class="text-5xl">{humidity}%</div>
					{#if labels?.humidity}<div class="text-lg text-neutral-400">{labels.humidity}</div>{/if}
				</div>
			{/if}
		</section>
	{/if}

	<section class="grid gap-4">
		<button
			type="button"
			class="flex h-28 items-center justify-center gap-4 rounded-3xl bg-amber-400 text-3xl font-semibold text-neutral-950 active:bg-amber-300"
			data-testid="touch-start-slideshow"
			onclick={onStart}
		>
			<PlayIcon class="size-10" fill="currentColor" />
			{fi.home.startSlideshow}
		</button>
		<button
			type="button"
			class="flex h-20 items-center justify-center gap-3 rounded-3xl bg-neutral-800 text-2xl font-medium active:bg-neutral-700"
			data-testid="touch-screen-off"
			onclick={screenOff}
		>
			<PowerIcon class="size-8" />
			{fi.home.screenOff}
		</button>
	</section>

	<section class="grid gap-2 text-xl text-neutral-300">
		{#if photoCount !== null}
			<p data-testid="touch-home-counts">{countsLine}</p>
		{/if}
		{#if sleep}
			<p>
				{sleep.idle_after_seconds > 0
					? fi.home.idleHint(formatSeconds(sleep.idle_after_seconds))
					: fi.home.idleNever}
			</p>
			{#if sleep.schedule}
				<p class="flex items-center gap-2">
					<MoonIcon class="size-6 shrink-0" />
					{fi.home.nightWindow(sleep.off_from, sleep.off_until)}
				</p>
			{/if}
		{/if}
	</section>
</div>
