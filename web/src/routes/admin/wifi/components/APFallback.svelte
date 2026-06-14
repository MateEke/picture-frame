<script lang="ts">
	import { onMount } from 'svelte';
	import { RouterIcon } from '@lucide/svelte';
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import { configureAP, apPasswordPayload } from '$lib/wifi';
	import type { WiFiState } from '$lib/api/types.gen';
	import SecretField from '$lib/SecretField.svelte';
	import { toaster } from '$lib/toaster';

	let {
		status,
		onsave
	}: {
		status: WiFiState;
		onsave: (newState: WiFiState) => void;
	} = $props();

	let apSSIDInput = $state('PictureFrame');
	let apPassword = $state('');
	// Cleared (isSet=false) means "open hotspot".
	let apPasswordSet = $state(false);
	let apToggleBusy = $state(false);

	onMount(() => {
		// Seed once from config, so later polls don't clobber edits.
		if (status.ap_ssid) apSSIDInput = status.ap_ssid;
		apPasswordSet = status.ap_has_password;
	});

	async function handleAPToggle(enabled: boolean) {
		apToggleBusy = true;
		const password = enabled ? apPasswordPayload(apPassword, apPasswordSet) : undefined;
		const newState = await configureAP(enabled, enabled ? apSSIDInput : '', password);
		apToggleBusy = false;
		if (newState) {
			onsave(newState);
			apPassword = '';
			apPasswordSet = newState.ap_has_password;
			toaster.success({ title: enabled ? 'AP fallback enabled' : 'AP fallback disabled' });
		}
	}
</script>

<div class="card bg-surface-100-900 reveal space-y-4 p-6" data-testid="wifi-ap">
	<div class="flex items-center gap-2">
		<RouterIcon class="text-primary-500 size-5" />
		<h2 class="h4">AP fallback</h2>
	</div>
	<p class="text-surface-500-400 text-sm">
		If the frame can't reach a known network it raises its own WiFi hotspot named
		<strong>{apSSIDInput}</strong>. Join it from your phone to reconnect the frame to WiFi.
	</p>

	<div class="flex items-center gap-4">
		<Switch
			checked={status.ap_enabled}
			disabled={apToggleBusy}
			onCheckedChange={({ checked }) => handleAPToggle(checked)}
			data-testid="ap-switch"
		>
			<Switch.HiddenInput />
			<Switch.Control><Switch.Thumb /></Switch.Control>
			<Switch.Label>{status.ap_enabled ? 'Enabled' : 'Disabled'}</Switch.Label>
		</Switch>
	</div>

	<form
		class="space-y-4"
		onsubmit={(e) => {
			e.preventDefault();
			if (apSSIDInput) handleAPToggle(true);
		}}
	>
		<div class="grid grid-cols-1 gap-3 md:grid-cols-2">
			<label class="label">
				<span class="label-text">Hotspot name (SSID)</span>
				<input
					class="input"
					type="text"
					bind:value={apSSIDInput}
					placeholder="PictureFrame"
					data-testid="ap-ssid"
				/>
			</label>
			<SecretField
				bind:value={apPassword}
				bind:isSet={apPasswordSet}
				wasSet={status.ap_has_password}
				label="Hotspot password"
				placeholder="No password (open hotspot)"
				warningText="Hotspot will become open (no password) on save."
				clearTestid="ap-password-clear"
			/>
		</div>
		<button
			type="submit"
			class="btn preset-tonal-primary"
			data-testid="ap-save"
			disabled={apToggleBusy || !apSSIDInput}
		>
			Save AP settings
		</button>
	</form>
</div>
