import { describe, it, expect, vi, afterEach } from 'vitest';
import { loadDevices } from './devices';

const mockGetSystemDevices = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiGetSystemDevices: (...args: unknown[]) => mockGetSystemDevices(...args)
}));

describe('devices', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	it('returns the device list on success', async () => {
		const body = { ble: [{ address: 'AA:BB', name: 'Sensor' }] };
		mockGetSystemDevices.mockResolvedValue({ data: body, error: undefined });

		expect(await loadDevices(fetch)).toEqual(body);
	});

	it('forwards fetch to the SDK call', async () => {
		const customFetch = vi.fn();
		mockGetSystemDevices.mockResolvedValue({ data: {}, error: undefined });

		await loadDevices(customFetch);

		expect(mockGetSystemDevices).toHaveBeenCalledWith({ fetch: customFetch });
	});

	it('degrades to null on a server error (no suggestions, never a broken page)', async () => {
		mockGetSystemDevices.mockResolvedValue({ data: undefined, error: { status: 500 } });

		expect(await loadDevices(fetch)).toBeNull();
	});

	it('degrades to null when the body is empty', async () => {
		mockGetSystemDevices.mockResolvedValue({ data: undefined, error: undefined });

		expect(await loadDevices(fetch)).toBeNull();
	});

	it('degrades to null when the request throws', async () => {
		mockGetSystemDevices.mockRejectedValue(new Error('network down'));

		expect(await loadDevices(fetch)).toBeNull();
	});
});
