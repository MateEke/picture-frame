<script lang="ts">
	let {
		message,
		confirmLabel,
		cancelLabel = 'Peruuta',
		onConfirm,
		onCancel
	}: {
		message: string;
		confirmLabel: string;
		cancelLabel?: string;
		onConfirm: () => void;
		onCancel: () => void;
	} = $props();
</script>

<div
	class="fixed inset-0 z-40 flex items-end bg-black/70"
	data-testid="touch-confirm"
	role="dialog"
	aria-modal="true"
>
	<div class="w-full rounded-t-3xl bg-neutral-900 p-8 pb-10">
		<p class="mb-8 text-3xl leading-snug font-medium">{message}</p>
		<div class="grid grid-cols-2 gap-4">
			<button
				type="button"
				class="h-20 rounded-2xl bg-neutral-800 text-2xl font-medium active:bg-neutral-700"
				data-testid="touch-confirm-cancel"
				onclick={onCancel}>{cancelLabel}</button
			>
			<button
				type="button"
				class="h-20 rounded-2xl bg-red-600 text-2xl font-semibold active:bg-red-500"
				data-testid="touch-confirm-ok"
				onclick={onConfirm}>{confirmLabel}</button
			>
		</div>
	</div>
</div>
