<script lang="ts">
	import { Dialog, Portal } from '@skeletonlabs/skeleton-svelte';
	import type { Snippet } from 'svelte';

	let {
		open,
		title,
		confirmLabel,
		confirmClass = 'preset-filled-error-500',
		busy = false,
		dialogTestid,
		confirmTestid,
		onconfirm,
		onclose,
		children
	}: {
		open: boolean;
		title: string;
		confirmLabel: string;
		/** Tailwind/Skeleton classes for the confirm button; defaults to a destructive red. */
		confirmClass?: string;
		/** While true the dialog can't be dismissed and both buttons are disabled. */
		busy?: boolean;
		dialogTestid?: string;
		confirmTestid?: string;
		onconfirm: () => void;
		onclose: () => void;
		children: Snippet;
	} = $props();
</script>

<Dialog
	{open}
	onOpenChange={(d: { open: boolean }) => {
		if (!d.open && !busy) onclose();
	}}
>
	<Portal>
		<Dialog.Backdrop class="fixed inset-0 z-50 bg-black/50" />
		<Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
			<Dialog.Content
				class="card bg-surface-100-900 w-full max-w-md space-y-4 p-6"
				data-testid={dialogTestid}
			>
				<Dialog.Title class="h4">{title}</Dialog.Title>
				<Dialog.Description>{@render children()}</Dialog.Description>
				<div class="flex justify-end gap-2">
					<button class="btn preset-tonal-surface" onclick={onclose} disabled={busy}>Cancel</button>
					<button
						class="btn {confirmClass}"
						data-testid={confirmTestid}
						onclick={onconfirm}
						disabled={busy}
					>
						{confirmLabel}
					</button>
				</div>
			</Dialog.Content>
		</Dialog.Positioner>
	</Portal>
</Dialog>
