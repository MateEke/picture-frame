<script lang="ts">
	import type { LibraryDto } from '$lib/api/types.gen';
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
		errors?: { share_url?: string };
	} = $props();
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
				label="Share URL"
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
		</div>
	{/if}
</div>
