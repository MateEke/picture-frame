<script lang="ts">
	import { Switch } from '@skeletonlabs/skeleton-svelte';
	import type { UpdaterDto } from '$lib/api/types.gen';
	import SecretField from '$lib/SecretField.svelte';
	import Field from './Field.svelte';

	let {
		updater = $bindable(),
		savedUpdater
	}: {
		updater: UpdaterDto;
		savedUpdater: UpdaterDto;
	} = $props();

	// 00:00–23:00, device-local, for the scheduled-check picker.
	const hours = Array.from({ length: 24 }, (_, h) => h);
	const pad = (h: number) => String(h).padStart(2, '0');
</script>

<div class="space-y-4">
	<p class="text-surface-500-400 text-sm">
		The frame checks for updates on its own; install them from the dashboard when one appears. These
		settings control automatic installs and where releases come from.
	</p>

	<div class="flex items-center justify-between gap-3">
		<div class="min-w-0">
			<p class="label-text">Automatic updates</p>
			<p class="text-surface-500-400 text-xs">
				Install same-version-line updates overnight. Takes effect after a restart.
			</p>
		</div>
		<Switch
			checked={updater.auto_update}
			onCheckedChange={({ checked }) => (updater.auto_update = checked)}
			data-testid="auto-update-switch"
		>
			<Switch.HiddenInput />
			<Switch.Label class="sr-only">Automatic updates</Switch.Label>
			<Switch.Control><Switch.Thumb /></Switch.Control>
		</Switch>
	</div>

	{#if updater.auto_update}
		<div class="border-surface-300-700 border-l-2 pl-4">
			<Field
				label="Check time"
				help="Hour of day (frame's local time) to check and install automatically."
				changed={updater.update_hour !== savedUpdater.update_hour}
				onrevert={() => (updater.update_hour = savedUpdater.update_hour)}
			>
				<select class="select" bind:value={updater.update_hour} data-testid="update-hour">
					{#each hours as h (h)}
						<option value={h}>{pad(h)}:00</option>
					{/each}
				</select>
			</Field>
		</div>
	{/if}

	<div class="border-surface-300-700 space-y-4 border-t pt-4">
		<p class="text-surface-500-400 text-xs">Advanced. Most frames never need these.</p>
		<Field
			label="Release source"
			help="GitHub repo (owner/name) to fetch releases from. Leave blank to track the official releases."
			changed={updater.github_repo !== savedUpdater.github_repo}
			onrevert={() => (updater.github_repo = savedUpdater.github_repo)}
		>
			<input
				class="input"
				type="text"
				bind:value={updater.github_repo}
				placeholder="owner/picture-frame"
				data-testid="github-repo"
			/>
		</Field>

		<SecretField
			label="Access token"
			placeholder="None (public repo)"
			warningText="Token will be removed on save."
			bind:value={updater.github_token}
			bind:isSet={updater.github_token_set}
			wasSet={savedUpdater.github_token_set}
			clearTestid="github-token-clear"
		/>
	</div>
</div>
