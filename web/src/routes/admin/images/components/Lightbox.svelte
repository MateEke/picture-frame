<script lang="ts">
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
	import { ExternalLinkIcon, Trash2Icon, XIcon } from '@lucide/svelte';

	import { immichPhotoUrl } from '$lib/images';

	let {
		name,
		albumId = null,
		immichLinks = {},
		canDelete,
		onClose,
		onDelete
	}: {
		name: string | null;
		albumId?: string | null;
		immichLinks?: { apiUrl?: string | null; shareUrl?: string | null };
		canDelete: boolean;
		onClose: () => void;
		onDelete: (name: string) => void;
	} = $props();

	const immichUrl = $derived(name ? immichPhotoUrl(immichLinks, name, albumId) : null);
</script>

<Dialog
	open={name !== null}
	onOpenChange={(d: { open: boolean }) => {
		if (!d.open) onClose();
	}}
>
	<Portal>
		<Dialog.Backdrop class="fixed inset-0 z-50 bg-black/70" />
		<Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
			<Dialog.Content class="w-full max-w-4xl space-y-3">
				{#if name}
					<div
						data-testid="lightbox"
						class="bg-surface-950 relative overflow-hidden rounded-lg shadow-2xl"
					>
						<img src="/img/{name}" alt={name} class="aspect-video w-full bg-black object-contain" />
						<button
							type="button"
							aria-label="Close"
							data-testid="lightbox-close"
							class="absolute top-2 right-2 grid size-9 place-items-center rounded-full bg-black/60 text-white shadow-lg backdrop-blur transition-colors hover:bg-black/80"
							onclick={onClose}
						>
							<XIcon class="size-5" />
						</button>
					</div>
					{#if immichUrl}
						<div class="flex justify-end">
							<a
								class="btn preset-filled-primary-500 flex items-center gap-1.5"
								data-testid="lightbox-immich"
								href={immichUrl}
								target="_blank"
								rel="noopener noreferrer"
							>
								<ExternalLinkIcon class="size-4" />Show in Immich
							</a>
						</div>
					{/if}
					{#if canDelete}
						<div class="flex justify-end">
							<button
								class="btn preset-filled-error-500 flex items-center gap-1.5"
								onclick={() => onDelete(name)}
							>
								<Trash2Icon class="size-4" />Delete photo
							</button>
						</div>
					{/if}
				{/if}
			</Dialog.Content>
		</Dialog.Positioner>
	</Portal>
</Dialog>
