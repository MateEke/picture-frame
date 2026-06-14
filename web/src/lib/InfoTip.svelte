<script lang="ts">
	import { InfoIcon } from '@lucide/svelte';

	let { text }: { text: string } = $props();
</script>

<!-- CSS-only: shows on mouse hover and on focus. Tapping the button focuses it,
     so it works on touch too, no JS state to desync, no transition to stick. -->
<span class="group relative inline-flex align-middle">
	<button type="button" class="text-surface-500 hover:text-surface-700-300" aria-label={text}>
		<InfoIcon class="size-3.5" />
	</button>
	<span
		role="tooltip"
		class="bg-surface-950-50 text-surface-50-950 pointer-events-none absolute bottom-full left-1/2 z-50 mb-1.5 hidden w-max max-w-56 -translate-x-1/2 rounded px-2.5 py-1.5 text-xs shadow-lg group-focus-within:block group-hover:block"
	>
		{text}
	</span>
</span>
