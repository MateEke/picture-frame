<script lang="ts">
	import { getSSEContext } from '$lib/sse.svelte';
	import { formatClockParts, formatMonthDay, formatWeekday, formatAlbumLabel } from '$lib/helpers';
	import { onMount } from 'svelte';
	import { overlayShiftPair } from './overlayShift';

	const sse = getSSEContext();

	const DEFAULT_LOCALE = 'en-US';
	let locale = $derived(sse.kiosk?.locale || DEFAULT_LOCALE);
	let timeZone = $derived(sse.kiosk?.timezone ?? '');
	let hideClockDate = $derived(sse.kiosk?.hide_clock_date ?? false);
	// A pair never mixes albums, so one label covers the whole slide; uploads
	// have no album and leave it empty.
	let album = $derived(sse.image?.album ?? '');
	let year = $derived(sse.image?.year ?? 0);
	let albumLabel = $derived(formatAlbumLabel(album, year));
	let showOverlay = $derived(!hideClockDate || album !== '');

	let now = $state(new Date());
	let clock = $derived(formatClockParts(now, locale, timeZone));
	let weekday = $derived(formatWeekday(now, locale, timeZone));
	let monthDay = $derived(formatMonthDay(now, locale, timeZone));
	// Rem scales with resolution (layout.css), so px must be recomputed, not cached.
	let rootPx = $state(16);
	let portrait = $state(false);
	let shift = $derived(overlayShiftPair(now, rootPx, portrait));

	onMount(() => {
		let timeout: ReturnType<typeof setTimeout>;

		const tick = () => {
			now = new Date();
			rootPx = parseFloat(getComputedStyle(document.documentElement).fontSize);
			portrait = matchMedia('(orientation: portrait)').matches;
			const msToNextMinute = 60_000 - (now.getSeconds() * 1000 + now.getMilliseconds());
			timeout = setTimeout(tick, msToNextMinute);
		};
		tick();

		return () => clearTimeout(timeout);
	});
</script>

{#if showOverlay}
	<div
		data-testid="kiosk-overlay"
		class="pointer-events-none fixed inset-x-0 bottom-0 flex items-end justify-between bg-linear-to-b from-transparent to-black/30 px-15 pt-33 pb-12 text-kiosk-fg text-shadow-lg/30 portrait:flex-col portrait:items-start portrait:gap-12"
	>
		{#if !hideClockDate}
			<div data-testid="kiosk-clock-block" style="transform: {shift.lead}">
				<div
					data-testid="kiosk-clock"
					class="flex items-baseline text-[10rem] leading-none font-medium tabular-nums"
				>
					<span>{clock.hours}</span>
					<span class="-mx-1 normal-nums">{clock.separator}</span>
					<span>{clock.minutes}</span>
					{#if clock.period}
						<span
							class={[
								'text-7xl font-semibold tracking-widest',
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

		{#if album}
			<div
				data-testid="kiosk-album"
				class="text-3xl font-semibold tracking-widest uppercase opacity-94 portrait:max-w-full"
				style="transform: {shift.trail}"
			>
				{albumLabel}
			</div>
		{/if}
	</div>
{/if}
