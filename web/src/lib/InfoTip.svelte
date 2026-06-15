<script lang="ts">
	import { Popover, Portal } from '@skeletonlabs/skeleton-svelte';
	import { InfoIcon } from '@lucide/svelte';

	let { text }: { text: string } = $props();
</script>

<Popover autoFocus={false} positioning={{ placement: 'top', gutter: 6 }}>
	<Popover.Trigger
		type="button"
		class="text-surface-500 hover:text-surface-700-300 inline-flex align-middle"
		aria-label={text}
	>
		<InfoIcon class="size-3.5" />
	</Popover.Trigger>
	<Portal>
		<Popover.Positioner>
			<Popover.Content
				class="bg-surface-950-50 text-surface-50-950 pointer-events-none z-50 max-w-56 rounded px-2.5 py-1.5 text-xs shadow-lg"
			>
				{text}
			</Popover.Content>
		</Popover.Positioner>
	</Portal>
</Popover>
