<script lang="ts">
	import type { PageProps } from './$types';
	import type { Component } from 'svelte';
	import { untrack } from 'svelte';
	import { Accordion } from '@skeletonlabs/skeleton-svelte';
	import {
		ChevronDownIcon,
		Undo2Icon,
		CircleAlertIcon,
		PencilIcon,
		MonitorIcon,
		ImagesIcon,
		CloudSunIcon,
		HouseIcon,
		RadioIcon,
		TerminalIcon,
		DownloadIcon,
		ShieldCheckIcon,
		ShieldOffIcon,
		BookOpenIcon,
		ExternalLinkIcon
	} from '@lucide/svelte';
	import { MANUAL_URL } from '$lib/links';
	import { saveConfig } from '$lib/config';
	import { createEmptyConfig, createEmptyMeta, configPayload, eq } from './utils';
	import { validate, sectionFromDetail } from './validate';
	import EssentialsCard from './components/EssentialsCard.svelte';
	import DisplayCard from './components/DisplayCard.svelte';
	import LibraryCard from './components/LibraryCard.svelte';
	import WeatherCard from './components/WeatherCard.svelte';
	import MqttCard from './components/MqttCard.svelte';
	import SensorsCard from './components/SensorsCard.svelte';
	import SystemCard from './components/SystemCard.svelte';
	import UpdatesCard from './components/UpdatesCard.svelte';
	import SecurityCard from './components/SecurityCard.svelte';
	import RestartConfirmDialog from './components/RestartConfirmDialog.svelte';

	let { data }: PageProps = $props();

	const securityOn = $derived(data.auth?.required ?? false);

	let draft = $state(createEmptyConfig());
	let savedConfig = $state(createEmptyConfig());
	let restartPending = $state(false);

	$effect(() => {
		const cfg = data.config;
		const temp = cfg ? structuredClone(cfg) : createEmptyConfig();
		// Secrets are write-only, so the server omits them; PasswordInput needs a string.
		temp.weather.api_key ??= '';
		temp.mqtt.password ??= '';
		temp.library.immich.share_password ??= '';
		temp.updater.github_token ??= '';
		untrack(() => {
			draft = temp;
			savedConfig = structuredClone(temp);
			restartPending = cfg?.restart_pending ?? false;
		});
	});

	const dirty = $derived(!eq(configPayload(draft), configPayload(savedConfig)));
	const validation = $derived(validate(draft));
	const valid = $derived(validation.issues.length === 0);
	// Per-section count so a collapsed panel still flags its errors.
	const issueCounts = $derived.by(() => {
		const counts: Record<string, number> = {};
		for (const issue of validation.issues) counts[issue.section] = (counts[issue.section] ?? 0) + 1;
		return counts;
	});
	let saving = $state(false);
	let saveError = $state<string | null>(null);
	let showRestartDialog = $state(false);
	let openSections = $state<string[]>([]);

	function openSection(value: string) {
		if (!openSections.includes(value)) openSections = [...openSections, value];
		// Scroll after the panel expands.
		requestAnimationFrame(() =>
			document
				.getElementById(`sec-${value}`)
				?.scrollIntoView({ behavior: 'smooth', block: 'center' })
		);
	}
	function jumpToFirstIssue() {
		const first = validation.issues[0];
		if (first) openSection(first.section);
	}

	let meta = $derived(data.meta);
	const adapters = $derived(data.devices?.bluetooth_adapters ?? []);
	const outputs = $derived(data.devices?.display_outputs ?? []);
	const logLevels = $derived(meta?.log_levels ?? ['debug', 'info', 'warn', 'error']);

	// Per-section dirty flags surface changes hidden inside collapsed panels.
	const displayDirty = $derived(
		draft.display.backend !== savedConfig.display.backend ||
			draft.display.output !== savedConfig.display.output
	);
	const libraryDirty = $derived(
		!eq(draft.library, savedConfig.library) ||
			draft.slideshow.images_dir !== savedConfig.slideshow.images_dir
	);
	const weatherDirty = $derived(!eq(draft.weather, savedConfig.weather));
	const mqttDirty = $derived(!eq(draft.mqtt, savedConfig.mqtt));
	const sensorsDirty = $derived(
		!eq(draft.sensors, savedConfig.sensors) ||
			draft.bluetooth_adapter !== savedConfig.bluetooth_adapter
	);
	const systemDirty = $derived(draft.log_level !== savedConfig.log_level);
	const updaterDirty = $derived(!eq(draft.updater, savedConfig.updater));
	// Colour the Updates section icon by availability, like Security's shield.
	const updaterIconClass = $derived.by(() => {
		const u = data.update;
		if (!u || !u.last_check_ok) return 'text-surface-500';
		return u.available ? 'text-primary-500' : 'text-success-500';
	});

	// Revert via $state.snapshot: a live $state proxy can't be structured-cloned, and
	// reassigning the plain clone re-proxies it so bound inputs update.
	function revertDisplay() {
		draft.display.backend = savedConfig.display.backend;
		draft.display.output = savedConfig.display.output;
	}
	function revertLibrary() {
		draft.library = $state.snapshot(savedConfig.library);
		draft.slideshow.images_dir = savedConfig.slideshow.images_dir;
	}
	function revertWeather() {
		draft.weather = $state.snapshot(savedConfig.weather);
	}
	function revertMqtt() {
		draft.mqtt = $state.snapshot(savedConfig.mqtt);
	}
	function revertSensors() {
		draft.sensors = $state.snapshot(savedConfig.sensors);
		draft.bluetooth_adapter = savedConfig.bluetooth_adapter;
	}
	function revertSystem() {
		draft.log_level = savedConfig.log_level;
	}
	function revertUpdater() {
		draft.updater = $state.snapshot(savedConfig.updater);
	}
	function discard() {
		draft = $state.snapshot(savedConfig);
	}

	async function handleSave() {
		saving = true;
		saveError = null;
		const result = await saveConfig(draft);
		saving = false;
		if (result.ok) {
			if (result.restart_pending) showRestartDialog = true;
			return;
		}
		saveError = result.detail;
		// Open the section the backend named.
		const section = sectionFromDetail(result.detail);
		if (section) openSection(section);
	}
</script>

{#snippet sectionHead(
	value: string,
	label: string,
	Icon: Component,
	sectionDirty: boolean,
	errorCount: number,
	revert: () => void,
	iconClass: string = 'text-primary-500'
)}
	<div id="sec-{value}" class="relative scroll-mt-20">
		<Accordion.ItemTrigger
			class="flex w-full items-center gap-3"
			data-testid="settings-section-{value}"
		>
			<Icon class="{errorCount > 0 ? 'text-error-500' : iconClass} size-5 shrink-0" />
			<span class="flex-1 text-left font-medium">{label}</span>
			<Accordion.ItemIndicator
				class="transition-transform duration-200 data-[state=open]:rotate-180"
			>
				<ChevronDownIcon class="size-5" />
			</Accordion.ItemIndicator>
		</Accordion.ItemTrigger>
		{#if errorCount > 0 || sectionDirty}
			<div
				class="pointer-events-none absolute top-1/2 right-12 flex -translate-y-1/2 items-center gap-2"
			>
				<!-- Error supersedes the modified hint; words collapse to icons on mobile. -->
				{#if errorCount > 0}
					<span
						class="badge preset-tonal-error flex items-center gap-1 text-xs"
						title="{errorCount} {errorCount === 1 ? 'issue' : 'issues'}"
					>
						<CircleAlertIcon class="size-3.5" />
						{errorCount}
						<span class="hidden md:inline">{errorCount === 1 ? 'issue' : 'issues'}</span>
					</span>
				{:else if sectionDirty}
					<span class="badge preset-tonal-primary flex items-center gap-1 text-xs" title="Modified">
						<PencilIcon class="size-3" />
						<span class="hidden md:inline">Modified</span>
					</span>
				{/if}
				{#if sectionDirty}
					<button
						type="button"
						class="text-surface-600-300 hover:text-primary-500 pointer-events-auto flex items-center gap-1 text-xs"
						onclick={revert}
						aria-label="Revert {label}"
						data-testid="settings-revert-{value}"
					>
						<Undo2Icon class="size-3.5" />
						<span class="hidden md:inline">Revert</span>
					</button>
				{/if}
			</div>
		{/if}
	</div>
{/snippet}

<div class="mx-auto max-w-3xl space-y-6 {dirty ? 'pb-20' : ''}">
	<header class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
		<div class="space-y-1">
			<h1 class="h2">Settings</h1>
			<p class="text-surface-500-400 text-sm">Tune how your frame looks and behaves.</p>
		</div>
		<a
			href={MANUAL_URL}
			target="_blank"
			rel="external noreferrer"
			class="btn btn-sm preset-tonal-surface hover:preset-tonal-primary w-fit shrink-0 gap-1.5"
			data-testid="settings-docs"
		>
			<BookOpenIcon class="size-4" />
			Manual
			<ExternalLinkIcon class="size-3.5 opacity-60" />
		</a>
	</header>

	{#if data.config === null}
		<div class="card bg-surface-100-900 p-6">
			<p class="text-error-500">Failed to load configuration. Check the server logs.</p>
		</div>
	{:else}
		{#if restartPending}
			<div
				class="card bg-warning-100-900 border-warning-500 flex items-center justify-between gap-4 border p-4"
			>
				<p class="text-sm font-medium">Some changes take effect after a restart.</p>
				<button
					class="btn btn-sm preset-tonal-warning shrink-0"
					data-testid="settings-restart-now"
					onclick={() => (showRestartDialog = true)}
				>
					Restart now
				</button>
			</div>
		{/if}

		<div class="reveal">
			<EssentialsCard
				bind:slideshow={draft.slideshow}
				bind:display={draft.display}
				savedSlideshow={savedConfig.slideshow}
				savedDisplay={savedConfig.display}
				sensors={draft.sensors}
			/>
		</div>

		<div class="card bg-surface-100-900 reveal p-2 delay-150">
			<Accordion
				value={openSections}
				onValueChange={(e) => (openSections = e.value)}
				multiple
				collapsible
			>
				<Accordion.Item value="display">
					{@render sectionHead(
						'display',
						'Display',
						MonitorIcon,
						displayDirty,
						issueCounts.display ?? 0,
						revertDisplay
					)}
					<Accordion.ItemContent>
						<div class="space-y-4 px-2 pt-0 pb-4 md:px-4">
							<DisplayCard
								bind:display={draft.display}
								savedDisplay={savedConfig.display}
								{outputs}
							/>
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>

				<Accordion.Item value="library">
					{@render sectionHead(
						'library',
						'Photo library',
						ImagesIcon,
						libraryDirty,
						issueCounts.library ?? 0,
						revertLibrary
					)}
					<Accordion.ItemContent>
						<div class="space-y-4 px-2 pt-0 pb-4 md:px-4">
							<LibraryCard
								bind:library={draft.library}
								bind:imagesDir={draft.slideshow.images_dir}
								savedLibrary={savedConfig.library}
								savedImagesDir={savedConfig.slideshow.images_dir}
								backends={meta?.backends ?? null}
								errors={validation.library}
							/>
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>

				<Accordion.Item value="weather">
					{@render sectionHead(
						'weather',
						'Weather',
						CloudSunIcon,
						weatherDirty,
						issueCounts.weather ?? 0,
						revertWeather
					)}
					<Accordion.ItemContent>
						<div class="space-y-4 px-2 pt-0 pb-4 md:px-4">
							<WeatherCard
								bind:weather={draft.weather}
								savedWeather={savedConfig.weather}
								units={meta?.units ?? null}
								errors={validation.weather}
							/>
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>

				<Accordion.Item value="mqtt">
					{@render sectionHead(
						'mqtt',
						'Home Assistant',
						HouseIcon,
						mqttDirty,
						issueCounts.mqtt ?? 0,
						revertMqtt
					)}
					<Accordion.ItemContent>
						<div class="space-y-4 px-2 pt-0 pb-4 md:px-4">
							<MqttCard
								bind:mqtt={draft.mqtt}
								savedMqtt={savedConfig.mqtt}
								errors={validation.mqtt}
							/>
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>

				<Accordion.Item value="sensors">
					{@render sectionHead(
						'sensors',
						'Sensors',
						RadioIcon,
						sensorsDirty,
						issueCounts.sensors ?? 0,
						revertSensors
					)}
					<Accordion.ItemContent>
						<div class="space-y-4 px-2 pt-0 pb-4 md:px-4">
							<SensorsCard
								bind:sensors={draft.sensors}
								bind:bluetoothAdapter={draft.bluetooth_adapter}
								savedBluetoothAdapter={savedConfig.bluetooth_adapter}
								{adapters}
								errors={validation.sensors}
								meta={meta ?? createEmptyMeta()}
							/>
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>

				<Accordion.Item value="system">
					{@render sectionHead(
						'system',
						'System',
						TerminalIcon,
						systemDirty,
						issueCounts.system ?? 0,
						revertSystem
					)}
					<Accordion.ItemContent>
						<div class="space-y-4 px-2 pt-0 pb-4 md:px-4">
							<SystemCard
								bind:logLevel={draft.log_level}
								savedLogLevel={savedConfig.log_level}
								levels={logLevels}
							/>
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>

				<Accordion.Item value="updates">
					{@render sectionHead(
						'updates',
						'Software updates',
						DownloadIcon,
						updaterDirty,
						0,
						revertUpdater,
						updaterIconClass
					)}
					<Accordion.ItemContent>
						<div class="space-y-4 px-2 pt-0 pb-4 md:px-4">
							<UpdatesCard bind:updater={draft.updater} savedUpdater={savedConfig.updater} />
						</div>
					</Accordion.ItemContent>
				</Accordion.Item>
			</Accordion>
		</div>
	{/if}

	<!-- Admin password: managed independently of the config-save flow above. -->
	<div class="card bg-surface-100-900 reveal p-2">
		<Accordion collapsible>
			<Accordion.Item value="security">
				<Accordion.ItemTrigger
					class="flex w-full items-center gap-3"
					data-testid="settings-section-security"
				>
					{#if securityOn}
						<ShieldCheckIcon class="text-success-500 size-5 shrink-0" />
					{:else}
						<ShieldOffIcon class="text-warning-500 size-5 shrink-0" />
					{/if}
					<span class="flex-1 text-left font-medium">Security</span>
					<Accordion.ItemIndicator
						class="transition-transform duration-200 data-[state=open]:rotate-180"
					>
						<ChevronDownIcon class="size-5" />
					</Accordion.ItemIndicator>
				</Accordion.ItemTrigger>
				<Accordion.ItemContent>
					<div class="px-2 pt-0 pb-4 md:px-4">
						<SecurityCard passwordSet={securityOn} />
					</div>
				</Accordion.ItemContent>
			</Accordion.Item>
		</Accordion>
	</div>
</div>

<!-- Sticky save bar, sits above the bottom nav on mobile -->
{#if dirty}
	<div
		data-testid="settings-save-bar"
		class="border-surface-300-700 bg-surface-50-950 fixed right-0 bottom-0 left-0 z-40 mb-16 flex items-center gap-3 border-t p-4 md:left-20 md:mb-0"
	>
		{#if saveError}
			<p
				data-testid="settings-save-error"
				class="text-error-500 flex-1 truncate text-sm"
				title={saveError}
			>
				{saveError}
			</p>
		{:else if !valid}
			<button
				type="button"
				data-testid="settings-issues"
				class="text-error-500 flex-1 text-left text-sm hover:underline"
				onclick={jumpToFirstIssue}
			>
				Fix {validation.issues.length} issue{validation.issues.length === 1 ? '' : 's'} to save
			</button>
		{:else}
			<p class="text-surface-500-400 flex-1 text-sm">Unsaved changes</p>
		{/if}
		<button
			data-testid="settings-discard"
			class="btn preset-tonal-surface shrink-0"
			onclick={discard}
			disabled={saving}
		>
			Discard
		</button>
		<button
			data-testid="settings-save"
			class="btn preset-tonal-primary shrink-0"
			onclick={handleSave}
			disabled={saving || !valid}
		>
			{saving ? 'Saving…' : 'Save'}
		</button>
	</div>
{/if}

{#if showRestartDialog}
	<RestartConfirmDialog oncancel={() => (showRestartDialog = false)} />
{/if}
