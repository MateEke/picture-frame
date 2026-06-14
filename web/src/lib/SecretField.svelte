<script lang="ts">
	import PasswordInput from '$lib/PasswordInput.svelte';

	let {
		value = $bindable(''),
		isSet = $bindable(false),
		wasSet,
		label,
		placeholder = 'Not set',
		warningText = 'Will be removed on save.',
		clearTestid
	}: {
		value?: string;
		isSet: boolean;
		wasSet: boolean;
		label: string;
		placeholder?: string;
		warningText?: string;
		clearTestid?: string;
	} = $props();
</script>

<div>
	<label class="label">
		<span class="label-text">{label}</span>
		<div class="flex w-full gap-2">
			<PasswordInput bind:value placeholder={isSet ? '••••••••' : placeholder} />
			{#if isSet}
				<button
					type="button"
					class="btn btn-sm preset-tonal-error shrink-0"
					data-testid={clearTestid}
					onclick={() => {
						isSet = false;
						value = '';
					}}
				>
					Clear
				</button>
			{/if}
		</div>
	</label>
	{#if wasSet && !isSet}
		<p class="text-warning-600 mt-1 text-xs">{warningText}</p>
	{/if}
</div>
