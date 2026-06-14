import { describe, it, expect, vi, afterEach } from 'vitest';
import { loadAuthStatus, login, logout, setPassword, loginRedirectTarget } from './auth';

const mockApiAuthStatus = vi.fn();
const mockApiAuthLogin = vi.fn();
const mockApiAuthLogout = vi.fn();
const mockApiAuthSetPassword = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiAuthStatus: (...args: unknown[]) => mockApiAuthStatus(...args),
	apiAuthLogin: (...args: unknown[]) => mockApiAuthLogin(...args),
	apiAuthLogout: (...args: unknown[]) => mockApiAuthLogout(...args),
	apiAuthSetPassword: (...args: unknown[]) => mockApiAuthSetPassword(...args)
}));

const mockToasterSuccess = vi.fn();
vi.mock('./toaster', () => ({
	toaster: { success: (...args: unknown[]) => mockToasterSuccess(...args), error: vi.fn() }
}));

describe('auth', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('loadAuthStatus', () => {
		it('returns the status on success', async () => {
			const authFetch = vi.fn();
			mockApiAuthStatus.mockResolvedValue({ data: { required: true, authenticated: true } });
			expect(await loadAuthStatus(authFetch)).toEqual({ required: true, authenticated: true });
			expect(mockApiAuthStatus).toHaveBeenCalledWith({ fetch: authFetch });
		});

		it('fails open to "not required" when status is unreachable', async () => {
			mockApiAuthStatus.mockResolvedValue({ data: undefined });
			expect(await loadAuthStatus(fetch)).toEqual({ required: false, authenticated: false });
		});
	});

	describe('login', () => {
		it('returns true when the server accepts the password', async () => {
			mockApiAuthLogin.mockResolvedValue({ error: undefined });
			expect(await login('pw')).toBe(true);
			expect(mockApiAuthLogin).toHaveBeenCalledWith({ body: { password: 'pw' } });
		});

		it('returns false when the server rejects the password', async () => {
			mockApiAuthLogin.mockResolvedValue({ error: { detail: 'invalid password' } });
			expect(await login('nope')).toBe(false);
		});
	});

	describe('logout', () => {
		it('calls the logout endpoint', async () => {
			mockApiAuthLogout.mockResolvedValue({ error: undefined });
			await logout();
			expect(mockApiAuthLogout).toHaveBeenCalled();
		});
	});

	describe('setPassword', () => {
		it('sets the first password and toasts "Password set"', async () => {
			mockApiAuthSetPassword.mockResolvedValue({ error: undefined });
			expect(await setPassword('', 'new')).toEqual({ ok: true });
			expect(mockApiAuthSetPassword).toHaveBeenCalledWith({ body: { current: '', new: 'new' } });
			expect(mockToasterSuccess).toHaveBeenCalledWith({ title: 'Password set' });
		});

		it('toasts "Password changed" when a current password is supplied', async () => {
			mockApiAuthSetPassword.mockResolvedValue({ error: undefined });
			expect(await setPassword('old', 'new')).toEqual({ ok: true });
			expect(mockToasterSuccess).toHaveBeenCalledWith({ title: 'Password changed' });
		});

		it('toasts "Password protection disabled" when clearing', async () => {
			mockApiAuthSetPassword.mockResolvedValue({ error: undefined });
			expect(await setPassword('old', '')).toEqual({ ok: true });
			expect(mockToasterSuccess).toHaveBeenCalledWith({ title: 'Password protection disabled' });
		});

		it('maps 403 to a field message without toasting success', async () => {
			mockApiAuthSetPassword.mockResolvedValue({ error: {}, response: { status: 403 } });
			expect(await setPassword('wrong', 'new')).toEqual({
				ok: false,
				message: 'Current password is incorrect.'
			});
			expect(mockToasterSuccess).not.toHaveBeenCalled();
		});

		it('maps other change failures to a generic update message', async () => {
			mockApiAuthSetPassword.mockResolvedValue({ error: {}, response: { status: 500 } });
			expect(await setPassword('old', 'new')).toEqual({
				ok: false,
				message: 'Could not update the password. Please try again.'
			});
		});

		it('maps disable failures to a disable-specific message', async () => {
			mockApiAuthSetPassword.mockResolvedValue({ error: {}, response: { status: 500 } });
			expect(await setPassword('old', '')).toEqual({
				ok: false,
				message: 'Could not disable protection. Please try again.'
			});
		});

		it('falls through to the generic message when no response object is present', async () => {
			mockApiAuthSetPassword.mockResolvedValue({ error: {}, response: undefined });
			expect(await setPassword('old', 'new')).toEqual({
				ok: false,
				message: 'Could not update the password. Please try again.'
			});
		});
	});

	describe('loginRedirectTarget', () => {
		it('ignores non-401 responses', () => {
			expect(
				loginRedirectTarget({ status: 200, url: 'http://f/api/config' }, '/admin', '')
			).toBeNull();
			expect(
				loginRedirectTarget({ status: 403, url: 'http://f/api/config' }, '/admin', '')
			).toBeNull();
		});

		it('ignores 401s from the auth endpoints (they own their 401s)', () => {
			expect(
				loginRedirectTarget({ status: 401, url: 'http://f/api/auth/login' }, '/admin', '')
			).toBeNull();
		});

		it('does not redirect when already on /login (prevents next= nesting)', () => {
			expect(
				loginRedirectTarget({ status: 401, url: 'http://f/api/config' }, '/login', '?next=%2Fadmin')
			).toBeNull();
		});

		it('redirects a gated 401 to /login with the encoded current path and query', () => {
			expect(
				loginRedirectTarget({ status: 401, url: 'http://f/api/config' }, '/admin/wifi', '?tab=ap')
			).toBe(`/login?next=${encodeURIComponent('/admin/wifi?tab=ap')}`);
		});
	});
});
