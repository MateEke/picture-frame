<script lang="ts">
	// A short-lived status line (saved, failed…) above the tab bar.
	let { text, tone = 'info' }: { text: string; tone?: 'info' | 'error' } = $props();
</script>

<div
	class={[
		'pointer-events-none absolute inset-x-6 bottom-6 z-30 rounded-2xl px-6 py-4 text-center text-xl font-medium shadow-lg',
		tone === 'error' ? 'bg-red-700 text-white' : 'bg-neutral-100 text-neutral-900'
	]}
	role="status"
	data-testid="touch-notice"
>
	{text}
</div>
