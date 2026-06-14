<script lang="ts">
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
	import { Trash2Icon, XIcon } from '@lucide/svelte';

	let {
		name,
		canDelete,
		onClose,
		onDelete
	}: {
		name: string | null;
		canDelete: boolean;
		onClose: () => void;
		onDelete: (name: string) => void;
	} = $props();
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
