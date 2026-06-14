<script lang="ts">
	import { ShieldOffIcon, TriangleAlertIcon, KeyRoundIcon } from '@lucide/svelte';
	import { invalidate } from '$app/navigation';
	import PasswordInput from '$lib/PasswordInput.svelte';
	import { setPassword } from '$lib/auth';

	let { passwordSet }: { passwordSet: boolean } = $props();

	// Reflect the prop until we successfully set/disable, then the override wins.
	let override = $state<boolean | null>(null);
	const enabled = $derived(override ?? passwordSet);
	let current = $state('');
	let newPw = $state('');
	let confirm = $state('');
	let busy = $state(false);
	let saveError = $state('');
	let disableError = $state('');

	const maxPasswordLength = 72;
	const mismatch = $derived(confirm.length > 0 && newPw !== confirm);
	const tooLong = $derived(newPw.length > maxPasswordLength);
	const canSave = $derived(
		!busy && newPw.length > 0 && !tooLong && newPw === confirm && (!enabled || current.length > 0)
	);

	function reset() {
		current = '';
		newPw = '';
		confirm = '';
		saveError = '';
		disableError = '';
	}

	async function save() {
		if (!canSave) return;
		saveError = '';
		busy = true;
		const result = await setPassword(enabled ? current : '', newPw);
		busy = false;
		if (result.ok) {
			override = true;
			reset();
			await invalidate('app:auth');
		} else {
			saveError = result.message;
		}
	}

	async function disable() {
		if (busy || current.length === 0) return;
		disableError = '';
		busy = true;
		const result = await setPassword(current, '');
		busy = false;
		if (result.ok) {
			override = false;
			reset();
			await invalidate('app:auth');
		} else {
			disableError = result.message;
		}
	}
</script>

<div class="space-y-5">
	{#if !enabled}
		<div class="card preset-tonal-warning flex items-center gap-2.5 px-3 py-2.5">
			<TriangleAlertIcon class="size-5 shrink-0" />
			<p class="text-sm">
				Unprotected. Anyone on your network or the recovery hotspot can change these settings.
			</p>
		</div>
	{/if}

	<div class="flex flex-col gap-4">
		{#if enabled}
			<label class="space-y-1">
				<span class="label-text">Current password</span>
				<PasswordInput
					bind:value={current}
					placeholder="Current password"
					disabled={busy}
					data-testid="security-current"
				/>
			</label>
		{/if}

		<label class="space-y-1">
			<span class="label-text">{enabled ? 'New password' : 'Password'}</span>
			<PasswordInput
				bind:value={newPw}
				placeholder="New password"
				maxlength={maxPasswordLength}
				disabled={busy}
				data-testid="security-new"
			/>
			{#if tooLong}
				<p class="text-error-500 text-xs" data-testid="security-too-long">
					Password must be at most {maxPasswordLength} characters.
				</p>
			{/if}
		</label>

		<label class="space-y-1">
			<span class="label-text">Confirm password</span>
			<PasswordInput
				bind:value={confirm}
				placeholder="Re-enter password"
				maxlength={maxPasswordLength}
				disabled={busy}
				data-testid="security-confirm"
			/>
			{#if mismatch}<p class="text-error-500 text-xs" data-testid="security-mismatch">
					Passwords do not match.
				</p>{/if}
		</label>
	</div>

	<div class="space-y-1.5">
		<button
			type="button"
			class="btn preset-filled-primary-500 w-full gap-2 sm:w-auto"
			disabled={!canSave}
			onclick={save}
			data-testid="security-save"
		>
			<KeyRoundIcon class="size-4" />
			{enabled ? 'Change password' : 'Set password'}
		</button>
		{#if saveError}<p class="text-error-500 text-sm" data-testid="security-save-error">
				{saveError}
			</p>{/if}
	</div>

	{#if enabled}
		<div class="border-surface-200-800 space-y-2 border-t pt-4">
			<p class="text-surface-700-300 text-sm font-medium">Disable password protection</p>
			<p class="text-surface-500-400 text-xs">
				Removes the password. The admin UI becomes open to anyone in range.
				<span class="text-warning-600-400 mt-1 block font-medium"
					>Requires your current password above.</span
				>
			</p>
			<button
				type="button"
				class="btn preset-tonal-error w-full gap-2 sm:w-auto"
				data-testid="security-disable"
				disabled={busy || current.length === 0}
				onclick={disable}
			>
				<ShieldOffIcon class="size-4" />
				Disable protection
			</button>
			{#if disableError}<p class="text-error-500 text-sm" data-testid="security-disable-error">
					{disableError}
				</p>{/if}
		</div>
	{/if}
</div>
