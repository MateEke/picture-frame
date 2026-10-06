<script lang="ts">
	import { apiListImmichAlbums } from '$lib/api/sdk.gen';
	import type { ImmichAlbum, LibraryDto } from '$lib/api/types.gen';
	import { DURATION_STOPS } from '$lib/duration';
	import SecretField from '$lib/SecretField.svelte';
	import DurationSlider from './DurationSlider.svelte';
	import Field from './Field.svelte';

	let {
		library = $bindable(),
		savedLibrary,
		backends,
		imagesDir = $bindable(),
		savedImagesDir,
		errors
	}: {
		library: LibraryDto;
		savedLibrary: LibraryDto;
		backends: string[] | null;
		imagesDir: string;
		savedImagesDir: string;
		errors?: { share_url?: string; url?: string; album_ids?: string };
	} = $props();

	// A url plus either the stored key or a freshly typed one is enough to list
	// albums; the endpoint is called with the draft values, so no save needed.
	const canList = $derived(
		Boolean(library.immich_api_key.url.trim()) &&
			(library.immich_api_key.api_key_set || Boolean(library.immich_api_key.api_key?.trim()))
	);

	// Derived from the saved share_url until the user picks a mode, so a reload of
	// a share-mode config opens on the right tab without an effect loop.
	let modeOverride = $state<'api' | 'share' | null>(null);
	let immichMode = $derived(modeOverride ?? (savedLibrary.immich.share_url ? 'share' : 'api'));

	// Switching modes clears the other one's fields, since config.Validate
	// rejects both configured at once. Only on an explicit pick — never on
	// mount, which would wipe a stored API key out of the draft.
	function selectMode(mode: 'api' | 'share') {
		modeOverride = mode;
		if (mode === 'api') library.immich.share_url = '';
		else {
			library.immich_api_key.url = '';
			library.immich_api_key.api_key = '';
			library.immich_api_key.api_key_set = false;
			library.immich_api_key.album_ids = [];
		}
	}

	let albums = $state<ImmichAlbum[] | null>(null);
	let listing = $state(false);
	let listError = $state('');
	let listed = $state(false);

	async function loadAlbums() {
		if (listing) return;
		listing = true;
		listError = '';
		try {
			// The typed url/key go in the body: the endpoint falls back to the
			// saved config for blanks, and a key must never ride in a query string.
			const { data, error } = await apiListImmichAlbums({
				body: {
					url: library.immich_api_key.url.trim(),
					api_key: library.immich_api_key.api_key?.trim() ?? ''
				}
			});
			if (error) {
				listError = error.detail || 'Could not load albums from Immich.';
				return;
			}
			albums = data ?? [];
			listed = true;
		} catch (e) {
			listError = e instanceof Error ? e.message : 'Could not load albums from Immich.';
		} finally {
			listing = false;
		}
	}

	// Selected order is the album's own order, so the merge on the backend
	// follows the order shown here.
	function toggle(id: string) {
		const current = library.immich_api_key.album_ids ?? [];
		library.immich_api_key.album_ids = current.includes(id)
			? current.filter((x) => x !== id)
			: [...current, id];
	}
</script>

<div class="space-y-4">
	<Field
		label="Backend"
		help="Choose fs to serve photos you upload, or immich to sync a shared Immich album."
		changed={library.backend !== savedLibrary.backend}
		onrevert={() => (library.backend = savedLibrary.backend)}
	>
		<select class="select" bind:value={library.backend} data-testid="library-backend">
			{#each backends ?? ['fs', 'immich'] as b (b)}
				<option value={b}>{b}</option>
			{/each}
		</select>
	</Field>

	<Field
		label="Images directory"
		help="Folder on the frame the photos are read from."
		changed={imagesDir !== savedImagesDir}
		onrevert={() => (imagesDir = savedImagesDir)}
	>
		<input class="input" type="text" bind:value={imagesDir} placeholder="images" />
	</Field>

	{#if library.backend === 'immich'}
		<div class="border-surface-300-700 space-y-4 border-l-2 pl-4">
			<Field
				label="Connection"
				help="Choose API key to pick albums from your Immich server, or Share link to use a single shared album. The two are mutually exclusive."
			>
				<select
					class="select"
					value={immichMode}
					onchange={(e) => selectMode(e.currentTarget.value as 'api' | 'share')}
					data-testid="library-immich-mode"
				>
					<option value="api">API key (pick albums)</option>
					<option value="share">Share link</option>
				</select>
			</Field>

			{#if immichMode === 'share'}
				<Field
					label="Share URL"
					help="Link to an Immich shared album. Both /share/… and custom /s/… links work."
					error={errors?.share_url}
					changed={library.immich.share_url !== savedLibrary.immich.share_url}
					onrevert={() => (library.immich.share_url = savedLibrary.immich.share_url)}
				>
					<input
						class="input"
						type="url"
						bind:value={library.immich.share_url}
						placeholder="https://immich.example.com/share/…"
						data-testid="library-share-url"
					/>
				</Field>
			{:else}
				<Field
					label="Immich URL"
					help="Base URL of your Immich server, without a trailing slash."
					error={errors?.url}
					changed={library.immich_api_key.url !== savedLibrary.immich_api_key.url}
					onrevert={() => (library.immich_api_key.url = savedLibrary.immich_api_key.url)}
				>
					<input
						class="input"
						type="url"
						bind:value={library.immich_api_key.url}
						placeholder="https://immich.example.com"
						data-testid="library-immich-url"
					/>
				</Field>
				<SecretField
					label="API key"
					placeholder="No API key set"
					warningText="API key will be removed on save."
					bind:value={library.immich_api_key.api_key}
					bind:isSet={library.immich_api_key.api_key_set}
					wasSet={savedLibrary.immich_api_key.api_key_set}
					clearTestid="library-immich-api-key-clear"
				/>
				<Field
					label="Albums"
					help="Pick every album the frame should show. Albums are merged in the order listed."
					error={errors?.album_ids}
					changed={JSON.stringify(library.immich_api_key.album_ids) !==
						JSON.stringify(savedLibrary.immich_api_key.album_ids)}
					onrevert={() =>
						(library.immich_api_key.album_ids = savedLibrary.immich_api_key.album_ids)}
				>
					<div class="space-y-2">
						<div class="flex flex-wrap items-center gap-2">
							<button
								type="button"
								class="btn btn-sm"
								onclick={loadAlbums}
								disabled={!canList || listing}
								data-testid="library-albums-load"
							>
								{listed ? 'Refresh albums' : 'Load albums'}
							</button>
							{#if listing}<span class="text-xs opacity-70">Loading…</span>{/if}
						</div>
						{#if !canList && !listed}
							<p class="text-xs opacity-70">Enter the URL and API key, then load your albums.</p>
						{/if}
						{#if listError}
							<p class="text-error-600 text-xs" data-testid="library-albums-error">{listError}</p>
						{/if}
						{#if albums}
							<ul class="max-h-64 space-y-1 overflow-y-auto" data-testid="library-albums-list">
								{#each albums as album (album.id)}
									{@const selected = (library.immich_api_key.album_ids ?? []).includes(album.id)}
									<li>
										<label class="flex cursor-pointer items-center gap-2 text-sm">
											<input
												type="checkbox"
												class="checkbox checkbox-sm"
												checked={selected}
												onchange={() => toggle(album.id)}
												data-testid="library-album-option"
											/>
											<span class="flex-1 truncate">{album.name}</span>
											<span class="text-xs opacity-70">{album.asset_count}</span>
										</label>
									</li>
								{:else}
									<li class="text-xs opacity-70">No albums visible to this API key.</li>
								{/each}
							</ul>
						{/if}
					</div>
				</Field>
			{/if}
			{#if immichMode === 'share'}
				<SecretField
					label="Share password"
					placeholder="No password set"
					warningText="Password will be removed on save."
					bind:value={library.immich.share_password}
					bind:isSet={library.immich.share_password_set}
					wasSet={savedLibrary.immich.share_password_set}
				/>
				<DurationSlider
					label="Sync interval"
					stops={DURATION_STOPS.immichSync}
					bind:value={library.immich.sync_interval}
					changed={library.immich.sync_interval !== savedLibrary.immich.sync_interval}
					onrevert={() => (library.immich.sync_interval = savedLibrary.immich.sync_interval)}
				/>
			{:else}
				<DurationSlider
					label="Sync interval"
					stops={DURATION_STOPS.immichSync}
					bind:value={library.immich_api_key.sync_interval}
					changed={library.immich_api_key.sync_interval !==
						savedLibrary.immich_api_key.sync_interval}
					onrevert={() => {
						library.immich_api_key.sync_interval = savedLibrary.immich_api_key.sync_interval;
					}}
				/>
			{/if}
		</div>
	{/if}
</div>
