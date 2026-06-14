import { describe, it, expect, vi, afterEach } from 'vitest';
import { loadSystemInfo } from './system';

const mockGetSystemInfo = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiGetSystemInfo: (...args: unknown[]) => mockGetSystemInfo(...args)
}));

describe('system', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	it('returns the system info on success', async () => {
		const body = { version: '1.2.3', hostname: 'frame', uptime: '3h', ip: '192.168.1.5' };
		mockGetSystemInfo.mockResolvedValue({ data: body, error: undefined });

		expect(await loadSystemInfo(fetch)).toEqual(body);
	});

	it('forwards fetch to the SDK call', async () => {
		const customFetch = vi.fn();
		mockGetSystemInfo.mockResolvedValue({ data: {}, error: undefined });

		await loadSystemInfo(customFetch);

		expect(mockGetSystemInfo).toHaveBeenCalledWith({ fetch: customFetch });
	});

	it('degrades to null on a server error (the card hides missing fields)', async () => {
		mockGetSystemInfo.mockResolvedValue({ data: undefined, error: { status: 500 } });

		expect(await loadSystemInfo(fetch)).toBeNull();
	});

	it('degrades to null when the body is empty', async () => {
		mockGetSystemInfo.mockResolvedValue({ data: undefined, error: undefined });

		expect(await loadSystemInfo(fetch)).toBeNull();
	});

	it('degrades to null when the request throws', async () => {
		mockGetSystemInfo.mockRejectedValue(new Error('network down'));

		expect(await loadSystemInfo(fetch)).toBeNull();
	});
});
