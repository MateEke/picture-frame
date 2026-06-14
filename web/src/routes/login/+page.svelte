<script lang="ts">
	import '$lib/admin-theme.css';
	import { page } from '$app/state';
	import { LockIcon, LogInIcon } from '@lucide/svelte';
	import PasswordInput from '$lib/PasswordInput.svelte';
	import { login } from '$lib/auth';

	let password = $state('');
	let failed = $state(false);
	let submitting = $state(false);

	function safeNext(): string {
		const next = page.url.searchParams.get('next');
		if (!next) return '/admin';
		// Same-origin only (no off-site redirect) and never /login (no sign-in loop).
		try {
			const url = new URL(next, location.origin);
			if (url.origin === location.origin && url.pathname !== '/login') {
				return url.pathname + url.search;
			}
		} catch {
			/* malformed → fall through */
		}
		return '/admin';
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		if (!password || submitting) return;
		submitting = true;
		failed = false;
		const ok = await login(password);
		if (ok) {
			// Full navigation so every load re-runs with the new session cookie.
			location.assign(safeNext());
		} else {
			submitting = false;
			failed = true;
			password = '';
		}
	}
</script>

<main class="grid min-h-svh place-items-center p-4">
	<form class="card bg-surface-100-900 flex w-full max-w-sm flex-col gap-5 p-6" onsubmit={submit}>
		<div class="flex flex-col items-center gap-2 text-center">
			<div class="bg-primary-500/15 text-primary-500 grid size-12 place-items-center rounded-full">
				<LockIcon class="size-6" />
			</div>
			<h1 class="text-xl font-semibold">Picture Frame</h1>
			<p class="text-surface-600-400 text-sm">Enter the admin password to continue.</p>
		</div>

		<label class="space-y-1">
			<span class="label-text">Password</span>
			<PasswordInput
				bind:value={password}
				placeholder="Admin password"
				data-testid="login-password"
			/>
		</label>

		{#if failed}
			<p data-testid="login-error" class="text-error-500 text-sm">
				Incorrect password. Please try again.
			</p>
		{/if}

		<button
			type="submit"
			data-testid="login-submit"
			class="btn preset-filled-primary-500 w-full gap-2"
			disabled={!password || submitting}
		>
			<LogInIcon class="size-4" />
			{submitting ? 'Signing in…' : 'Sign in'}
		</button>
	</form>
</main>
