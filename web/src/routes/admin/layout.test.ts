import { describe, it, expect, vi, beforeEach } from 'vitest';
import { load } from './+layout';

const mockLoadAuthStatus = vi.fn();
vi.mock('$lib/auth', () => ({
	loadAuthStatus: (...args: unknown[]) => mockLoadAuthStatus(...args)
}));

// SvelteKit's LayoutLoadEvent can't be structurally narrowed, so we build the
// few fields the loader actually reads and cast once here.
function event(pathname: string, search = '') {
	const ev = {
		fetch: vi.fn(),
		url: new URL(`http://frame.local${pathname}${search}`),
		depends: vi.fn()
	};
	return ev as unknown as Parameters<typeof load>[0] & typeof ev;
}

describe('admin layout load', () => {
	beforeEach(() => vi.clearAllMocks());

	it('exposes auth status and registers the app:auth dependency', async () => {
		mockLoadAuthStatus.mockResolvedValue({ required: true, authenticated: true });
		const ev = event('/admin');

		expect(await load(ev)).toEqual({ auth: { required: true, authenticated: true } });
		expect(ev.depends).toHaveBeenCalledWith('app:auth');
	});

	it('does not redirect when auth is not required', async () => {
		mockLoadAuthStatus.mockResolvedValue({ required: false, authenticated: false });
		await expect(load(event('/admin/settings'))).resolves.toEqual({
			auth: { required: false, authenticated: false }
		});
	});

	it('does not redirect when required and already authenticated', async () => {
		mockLoadAuthStatus.mockResolvedValue({ required: true, authenticated: true });
		await expect(load(event('/admin'))).resolves.toBeDefined();
	});

	it('redirects to login with an encoded next when required but unauthenticated', async () => {
		mockLoadAuthStatus.mockResolvedValue({ required: true, authenticated: false });

		await expect(load(event('/admin/wifi', '?tab=ap'))).rejects.toMatchObject({
			status: 307,
			location: `/login?next=${encodeURIComponent('/admin/wifi?tab=ap')}`
		});
	});
});
