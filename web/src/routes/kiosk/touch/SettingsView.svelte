<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import {
		fetchDeviceInfo,
		fetchSettings,
		formatSeconds,
		parseGoDuration,
		saveSettings,
		shiftClock,
		stepChoice,
		toGoDuration,
		type DeviceInfo,
		type TouchSettingsBody,
		type TouchSettingsDto
	} from '$lib/touch';
	import { fi } from './fi';
	import Notice from './Notice.svelte';
	import Stepper from './Stepper.svelte';
	import Toggle from './Toggle.svelte';

	const INTERVALS = [10, 15, 30, 60, 120, 300, 600, 1800, 3600] as const;
	const IDLE = [0, 30, 60, 120, 300, 600, 1800] as const;
	const WAKE = [60, 120, 300, 600, 1800] as const;
	const BRIGHTNESS = [0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100] as const;
	const ROTATIONS = [0, 90, 180, 270] as const;
	const SAVE_DEBOUNCE_MS = 700;

	let settings = $state<TouchSettingsBody | null>(null);
	let loadFailed = $state(false);
	let info = $state<DeviceInfo | null>(null);
	let status = $state<{ text: string; tone: 'info' | 'error' } | null>(null);
	let saveTimer: ReturnType<typeof setTimeout> | undefined;
	let statusTimer: ReturnType<typeof setTimeout> | undefined;

	onMount(() => {
		fetchSettings().then((s) => {
			if (s) settings = s;
			else loadFailed = true;
		});
		fetchDeviceInfo().then((d) => (info = d));
	});

	onDestroy(() => {
		// Leaving the tab mid-debounce still saves.
		if (saveTimer !== undefined) {
			clearTimeout(saveTimer);
			void flush();
		}
		clearTimeout(statusTimer);
	});

	function show(text: string, tone: 'info' | 'error' = 'info') {
		status = { text, tone };
		clearTimeout(statusTimer);
		statusTimer = setTimeout(() => (status = null), 2500);
	}

	async function flush() {
		saveTimer = undefined;
		if (!settings) return;
		const body: TouchSettingsDto = {
			sleep: settings.sleep,
			interval: settings.interval,
			randomize: settings.randomize,
			split_screen: settings.split_screen,
			brightness: settings.brightness,
			rotation: settings.rotation
		};
		const err = await saveSettings(body);
		if (err) show(`${fi.settings.saveFailed}: ${err}`, 'error');
		else show(fi.settings.saved);
	}

	// Every change saves itself after a short pause, so there's no Save button to forget.
	function edit(fn: (s: TouchSettingsBody) => void) {
		if (!settings) return;
		fn(settings);
		clearTimeout(saveTimer);
		saveTimer = setTimeout(flush, SAVE_DEBOUNCE_MS);
	}

	const interval = $derived(settings ? parseGoDuration(settings.interval) : 0);
	const idleAfter = $derived(settings ? parseGoDuration(settings.sleep.idle_after) : 0);
	const wakeFor = $derived(settings ? parseGoDuration(settings.sleep.wake_for) : 0);
</script>

<div class="h-full overflow-y-auto overscroll-contain px-6 pb-10" data-testid="touch-settings">
	{#if loadFailed}
		<p class="p-4 text-2xl text-red-300">{fi.settings.loadFailed}</p>
	{:else if settings}
		<section class="border-b border-neutral-800 py-4">
			<h2 class="mb-2 text-2xl font-semibold text-amber-300">{fi.settings.slideshow}</h2>
			<Stepper
				label={fi.settings.interval}
				value={formatSeconds(interval)}
				testId="touch-set-interval"
				canDown={interval > INTERVALS[0]}
				canUp={interval < INTERVALS[INTERVALS.length - 1]}
				onStep={(d) => edit((s) => (s.interval = toGoDuration(stepChoice(INTERVALS, interval, d))))}
			/>
			<Toggle
				label={fi.settings.randomize}
				checked={settings.randomize}
				testId="touch-set-randomize"
				onChange={(v) => edit((s) => (s.randomize = v))}
			/>
			<Toggle
				label={fi.settings.split}
				hint={fi.settings.splitHint}
				checked={settings.split_screen}
				onChange={(v) => edit((s) => (s.split_screen = v))}
			/>
		</section>

		<section class="border-b border-neutral-800 py-4">
			<h2 class="mb-2 text-2xl font-semibold text-amber-300">{fi.settings.sleep}</h2>
			<Stepper
				label={fi.settings.idleAfter}
				value={formatSeconds(idleAfter)}
				testId="touch-set-idle"
				canDown={idleAfter > 0}
				canUp={idleAfter < IDLE[IDLE.length - 1]}
				onStep={(d) =>
					edit((s) => (s.sleep.idle_after = toGoDuration(stepChoice(IDLE, idleAfter, d))))}
			/>
		</section>

		<section class="border-b border-neutral-800 py-4">
			<h2 class="mb-2 text-2xl font-semibold text-amber-300">{fi.settings.night}</h2>
			<Toggle
				label={fi.settings.nightEnabled}
				checked={settings.sleep.schedule}
				testId="touch-set-night"
				onChange={(v) => edit((s) => (s.sleep.schedule = v))}
			/>
			{#if settings.sleep.schedule}
				<Stepper
					label={fi.settings.offFrom}
					value={settings.sleep.off_from}
					testId="touch-set-off-from"
					onStep={(d) =>
						edit((s) => (s.sleep.off_from = shiftClock(s.sleep.off_from || '23:00', d * 15)))}
				/>
				<Stepper
					label={fi.settings.offUntil}
					value={settings.sleep.off_until}
					testId="touch-set-off-until"
					onStep={(d) =>
						edit((s) => (s.sleep.off_until = shiftClock(s.sleep.off_until || '07:00', d * 15)))}
				/>
				<Stepper
					label={fi.settings.wakeFor}
					value={formatSeconds(wakeFor)}
					canDown={wakeFor > WAKE[0]}
					canUp={wakeFor < WAKE[WAKE.length - 1]}
					onStep={(d) =>
						edit((s) => (s.sleep.wake_for = toGoDuration(stepChoice(WAKE, wakeFor, d))))}
				/>
			{/if}
		</section>

		{#if settings.brightness_supported || settings.rotation_supported}
			<section class="border-b border-neutral-800 py-4">
				<h2 class="mb-2 text-2xl font-semibold text-amber-300">{fi.settings.screen}</h2>
				{#if settings.brightness_supported}
					<Stepper
						label={fi.settings.brightness}
						value={settings.brightness > 0
							? `${settings.brightness} %`
							: fi.settings.brightnessUnset}
						testId="touch-set-brightness"
						canDown={settings.brightness > 0}
						canUp={settings.brightness < 100}
						onStep={(d) => edit((s) => (s.brightness = stepChoice(BRIGHTNESS, s.brightness, d)))}
					/>
				{/if}
				{#if settings.rotation_supported}
					<div class="flex items-center gap-4 py-3">
						<span class="flex-1 text-xl">{fi.settings.rotation}</span>
						<div class="flex gap-2">
							{#each ROTATIONS as r (r)}
								<button
									type="button"
									class={[
										'h-16 w-20 rounded-2xl text-xl font-semibold',
										settings.rotation === r
											? 'bg-amber-400 text-neutral-950'
											: 'bg-neutral-800 active:bg-neutral-700'
									]}
									onclick={() => edit((s) => (s.rotation = r))}>{r}°</button
								>
							{/each}
						</div>
					</div>
				{/if}
			</section>
		{/if}

		<section class="py-4 text-xl">
			<h2 class="mb-3 text-2xl font-semibold text-amber-300">{fi.settings.about}</h2>
			{#if info}
				<dl class="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2">
					<dt class="text-neutral-400">{fi.settings.address}</dt>
					<dd>{info.ip || '—'}</dd>
					<dt class="text-neutral-400">{fi.settings.network}</dt>
					<dd>{info.ssid || '—'}</dd>
					<dt class="text-neutral-400">{fi.settings.version}</dt>
					<dd>{info.version || '—'}</dd>
				</dl>
			{/if}
			<p class="mt-4 text-lg text-neutral-400">{fi.settings.adminHint}</p>
		</section>
	{/if}

	{#if status}
		<Notice text={status.text} tone={status.tone} />
	{/if}
</div>
