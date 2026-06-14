import { apiAuthStatus, apiAuthLogin, apiAuthLogout, apiAuthSetPassword } from '$lib/api/sdk.gen';
import type { AuthStatusResponse } from '$lib/api/types.gen';
import { toaster } from './toaster';

// Fails open to "not required" so a backend hiccup can't hard-lock the UI.
export async function loadAuthStatus(fetch: typeof globalThis.fetch): Promise<AuthStatusResponse> {
	const { data } = await apiAuthStatus({ fetch });
	return data ?? { required: false, authenticated: false };
}

export async function login(password: string): Promise<boolean> {
	const { error } = await apiAuthLogin({ body: { password } });
	return !error;
}

export async function logout(): Promise<void> {
	await apiAuthLogout();
}

// Decides whether a 401 response should bounce the user to /login, and to where.
// Skips /api/auth/* (those own their 401s) and /login itself (else next= nests and
// traps the user after sign-in). Pure so it can be unit-tested away from the browser.
export function loginRedirectTarget(
	response: { status: number; url: string },
	pathname: string,
	search: string
): string | null {
	if (response.status !== 401) return null;
	if (response.url.includes('/api/auth/')) return null;
	if (pathname === '/login') return null;
	return `/login?next=${encodeURIComponent(pathname + search)}`;
}

export type SetPasswordResult = { ok: true } | { ok: false; message: string };

function successTitle(disabling: boolean, hasCurrent: boolean): string {
	if (disabling) return 'Password protection disabled';
	return hasCurrent ? 'Password changed' : 'Password set';
}

// Sets/changes/disables (empty newPassword) the admin password. Toasts success;
// returns an inline message on failure.
export async function setPassword(
	current: string,
	newPassword: string
): Promise<SetPasswordResult> {
	const disabling = newPassword === '';
	const { error, response } = await apiAuthSetPassword({ body: { current, new: newPassword } });
	if (!error) {
		toaster.success({ title: successTitle(disabling, current !== '') });
		return { ok: true };
	}
	if (response?.status === 403) {
		return { ok: false, message: 'Current password is incorrect.' };
	}
	return {
		ok: false,
		message: disabling
			? 'Could not disable protection. Please try again.'
			: 'Could not update the password. Please try again.'
	};
}
