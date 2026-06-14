<script lang="ts">
	import { EyeIcon, EyeOffIcon } from '@lucide/svelte';
	import type { HTMLInputAttributes } from 'svelte/elements';

	// Forward arbitrary input attributes (placeholder, disabled, data-testid, …);
	// the controlled type/class/autocomplete below always win.
	let { value = $bindable(''), ...rest }: { value?: string } & Omit<HTMLInputAttributes, 'value'> =
		$props();

	let visible = $state(false);
</script>

<div class="relative w-full">
	<input
		{...rest}
		class="input pr-10"
		type={visible ? 'text' : 'password'}
		bind:value
		autocomplete="off"
	/>
	<button
		type="button"
		class="text-surface-500 hover:text-surface-700-300 absolute inset-y-0 right-0 flex items-center px-3"
		onclick={() => (visible = !visible)}
		aria-label={visible ? 'Hide password' : 'Show password'}
		tabindex="-1"
	>
		{#if visible}<EyeOffIcon class="size-4" />{:else}<EyeIcon class="size-4" />{/if}
	</button>
</div>
