<script lang="ts">
	import { untrack } from 'svelte';
	import { slide } from 'svelte/transition';
	import { prefersReducedMotion } from 'svelte/motion';
	import {
		splitDuration,
		checkDurationParts,
		durationHint,
		isDurationUnit,
		DURATION_UNITS,
		type DurationScale
	} from '#lib/duration.js';

	let {
		value,
		scale,
		label,
		zeroLabel,
		testid,
		onwrite,
		onclose
	}: {
		value: string;
		scale: DurationScale;
		label: string;
		zeroLabel: string;
		testid: string;
		onwrite: (value: string) => void;
		onclose: (returnFocus: boolean) => void;
	} = $props();

	const noteId = $props.id();
	// Mounted per edit, so the value at mount is what Esc restores.
	const original = untrack(() => value);
	const initial = untrack(() => splitDuration(original, scale));

	let amount = $state(initial.amount);
	let unit = $state(initial.unit);
	let error = $state<string | null>(null);
	let group: HTMLElement | undefined = $state();

	const hint = $derived(durationHint(scale, zeroLabel));

	// Errors wait for finish() so a half-typed number doesn't flash one.
	function applyLive() {
		const result = checkDurationParts(amount, unit, scale);
		if ('error' in result) return;
		error = null;
		onwrite(result.value);
	}

	function finish(returnFocus: boolean) {
		// The original may sit outside the limits (e.g. hand-edited config).
		if (amount === initial.amount && unit === initial.unit) {
			onwrite(original);
			return onclose(returnFocus);
		}
		const result = checkDurationParts(amount, unit, scale);
		if ('value' in result) return onclose(returnFocus);
		// Don't leave a half-typed valid value behind for Save to pick up.
		onwrite(original);
		error = result.error;
	}

	function onkeydown(e: KeyboardEvent) {
		if (e.key !== 'Enter') return;
		e.preventDefault();
		finish(true);
	}

	// Deferred: iOS may report a null relatedTarget when the select is tapped.
	function onfocusout() {
		setTimeout(() => {
			if (group?.isConnected && !group.contains(document.activeElement)) finish(false);
		});
	}

	// Window capture runs before a dialog's document-level Esc handler.
	function onWindowKeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape' || !(e.target instanceof Node) || !group?.contains(e.target)) return;
		e.preventDefault();
		e.stopPropagation();
		onwrite(original);
		onclose(true);
	}

	function focusAndSelect(input: HTMLInputElement) {
		input.focus();
		input.select();
	}
</script>

<svelte:window onkeydowncapture={onWindowKeydown} />

<div
	class="flex flex-wrap items-center gap-x-3 gap-y-1 pb-2.5"
	in:slide={{ duration: prefersReducedMotion.current ? 0 : 150 }}
>
	<div
		bind:this={group}
		class="field-group grid-cols-[4.5rem_5rem] gap-0"
		role="group"
		aria-label="Custom {label}"
		{onfocusout}
	>
		<input
			class="input aria-invalid:ring-error-500 h-9 px-2 py-0 text-right text-base tabular-nums sm:h-8 sm:text-sm"
			type="text"
			inputmode="numeric"
			enterkeyhint="done"
			autocomplete="off"
			aria-label="{label}, amount"
			aria-invalid={error !== null}
			aria-describedby={noteId}
			bind:value={() => amount, (v) => ((amount = v), applyLive())}
			{onkeydown}
			{@attach focusAndSelect}
			data-testid="{testid}-amount"
		/>
		<!-- The 1px overlap merges both inset rings into one seam. -->
		<select
			class="select -ms-px h-9 py-0 text-base sm:h-8 sm:text-sm"
			aria-label="{label}, unit"
			{onkeydown}
			bind:value={() => unit, (v) => isDurationUnit(v) && ((unit = v), applyLive())}
			data-testid="{testid}-unit"
		>
			{#each DURATION_UNITS as u (u)}
				<option value={u}>{u}</option>
			{/each}
		</select>
	</div>
	<div aria-live="polite">
		{#if error}
			<p id={noteId} class="text-error-500 text-xs" data-testid="{testid}-error">{error}</p>
		{:else}
			<p id={noteId} class="text-surface-600-400 text-xs" data-testid="{testid}-hint">{hint}</p>
		{/if}
	</div>
</div>
