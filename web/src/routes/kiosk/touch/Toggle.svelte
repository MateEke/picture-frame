<script lang="ts">
	let {
		label,
		hint,
		checked,
		onChange,
		testId
	}: {
		label: string;
		hint?: string;
		checked: boolean;
		onChange: (v: boolean) => void;
		testId?: string;
	} = $props();
</script>

<button
	type="button"
	role="switch"
	aria-checked={checked}
	class="flex w-full items-center gap-4 py-4 text-left"
	data-testid={testId}
	onclick={() => onChange(!checked)}
>
	<span class="min-w-0 flex-1">
		<span class="block text-xl leading-snug">{label}</span>
		{#if hint}<span class="block text-base text-neutral-400">{hint}</span>{/if}
	</span>
	<span
		class={[
			'relative h-11 w-20 shrink-0 rounded-full transition-colors',
			checked ? 'bg-amber-400' : 'bg-neutral-700'
		]}
	>
		<span
			class="absolute top-1 left-1 size-9 rounded-full bg-white shadow transition-transform"
			style="transform: translateX({checked ? '2.25rem' : '0'})"
		></span>
	</span>
</button>
