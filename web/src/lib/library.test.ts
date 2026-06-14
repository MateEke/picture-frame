import { describe, it, expect, vi, afterEach } from 'vitest';
import { loadLibrary, syncLibrary } from './library';

const mockGetLibrary = vi.fn();
const mockSyncLibrary = vi.fn();
const mockInvalidate = vi.fn();
const mockToasterError = vi.fn();
const mockToasterSuccess = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiGetLibrary: (...args: unknown[]) => mockGetLibrary(...args),
	apiSyncLibrary: (...args: unknown[]) => mockSyncLibrary(...args)
}));

vi.mock('$app/navigation', () => ({
	invalidate: (...args: unknown[]) => mockInvalidate(...args)
}));

vi.mock('./toaster', () => ({
	toaster: {
		error: (...args: unknown[]) => mockToasterError(...args),
		success: (...args: unknown[]) => mockToasterSuccess(...args)
	}
}));

describe('library', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	it('returns parsed fs response', async () => {
		mockGetLibrary.mockResolvedValue({ data: { backend: 'fs' }, error: undefined });

		const info = await loadLibrary(fetch);
		expect(info).toEqual({ backend: 'fs' });
	});

	it('returns parsed immich response with sync', async () => {
		mockGetLibrary.mockResolvedValue({
			data: {
				backend: 'immich',
				sync: { last_sync: '2026-05-28T12:00:00Z', asset_count: 47 }
			},
			error: undefined
		});

		const info = await loadLibrary(fetch);
		expect(info?.backend).toBe('immich');
		expect(info?.sync?.asset_count).toBe(47);
	});

	it('passes fetch to the SDK call', async () => {
		const customFetch = vi.fn();
		mockGetLibrary.mockResolvedValue({ data: { backend: 'fs' }, error: undefined });

		await loadLibrary(customFetch);

		expect(mockGetLibrary).toHaveBeenCalledWith(expect.objectContaining({ fetch: customFetch }));
	});

	it('throws on error', async () => {
		mockGetLibrary.mockResolvedValue({ data: undefined, error: { status: 500 } });

		await expect(loadLibrary(fetch)).rejects.toThrow('Failed to load library info');
	});

	const fast = { pollMs: 1, maxPolls: 5 };

	it('syncLibrary waits for a fresh sync, then refreshes and reports the count', async () => {
		mockSyncLibrary.mockResolvedValue({ error: undefined });
		mockGetLibrary.mockResolvedValue({
			data: { backend: 'immich', sync: { last_sync: '2026-06-05T10:00:00Z', asset_count: 12 } },
			error: undefined
		});

		const ok = await syncLibrary('2026-06-05T09:00:00Z', fast);

		expect(ok).toBe(true);
		expect(mockToasterSuccess).toHaveBeenCalledWith({
			title: 'Sync complete',
			description: '12 photos in your album'
		});
		expect(mockInvalidate).toHaveBeenCalledWith('/api/library');
		expect(mockInvalidate).toHaveBeenCalledWith('/api/images');
	});

	it('syncLibrary reports a sync that finished with an error', async () => {
		mockSyncLibrary.mockResolvedValue({ error: undefined });
		mockGetLibrary.mockResolvedValue({
			data: {
				backend: 'immich',
				sync: { last_sync: '2026-06-05T10:00:00Z', asset_count: 0, last_error: 'status 401' }
			},
			error: undefined
		});

		const ok = await syncLibrary('2026-06-05T09:00:00Z', fast);

		expect(ok).toBe(false);
		expect(mockToasterError).toHaveBeenCalledWith({
			title: 'Sync finished with errors',
			description: 'status 401'
		});
		// Even on a failed sync we refresh so the latest status/photos are shown.
		expect(mockInvalidate).toHaveBeenCalledWith('/api/images');
	});

	it('syncLibrary degrades gracefully when the sync outruns the poll budget', async () => {
		mockSyncLibrary.mockResolvedValue({ error: undefined });
		// last_sync never advances past prev → no fresh attempt within the budget.
		mockGetLibrary.mockResolvedValue({
			data: { backend: 'immich', sync: { last_sync: '2026-06-05T09:00:00Z', asset_count: 3 } },
			error: undefined
		});

		const ok = await syncLibrary('2026-06-05T09:00:00Z', fast);

		expect(ok).toBe(true);
		expect(mockToasterSuccess).toHaveBeenCalledWith({
			title: 'Sync started',
			description: 'New photos will appear shortly.'
		});
		expect(mockInvalidate).toHaveBeenCalledWith('/api/images');
	});

	it('syncLibrary reports an error and skips polling on a failed trigger', async () => {
		mockSyncLibrary.mockResolvedValue({ error: { status: 409 } });

		const ok = await syncLibrary('2026-06-05T09:00:00Z', fast);

		expect(ok).toBe(false);
		expect(mockToasterError).toHaveBeenCalledWith({
			title: 'Could not start sync',
			description: 'Server returned an error'
		});
		expect(mockGetLibrary).not.toHaveBeenCalled();
		expect(mockInvalidate).not.toHaveBeenCalled();
	});

	it('polls exactly maxPolls times before giving up', async () => {
		mockSyncLibrary.mockResolvedValue({ error: undefined });
		mockGetLibrary.mockResolvedValue({
			data: { backend: 'immich', sync: { last_sync: '2026-06-05T09:00:00Z', asset_count: 3 } },
			error: undefined
		});

		await syncLibrary('2026-06-05T09:00:00Z', { pollMs: 1, maxPolls: 5 });

		expect(mockGetLibrary).toHaveBeenCalledTimes(5);
	});

	it('survives a transient poll with no data or no sync, then succeeds', async () => {
		mockSyncLibrary.mockResolvedValue({ error: undefined });
		mockGetLibrary
			.mockResolvedValueOnce({ data: undefined, error: { status: 503 } }) // data?.sync guard
			.mockResolvedValueOnce({ data: { backend: 'immich' }, error: undefined }) // sync?.last_sync guard
			.mockResolvedValue({
				data: { backend: 'immich', sync: { last_sync: '2026-06-05T10:00:00Z', asset_count: 7 } },
				error: undefined
			});

		const ok = await syncLibrary('2026-06-05T09:00:00Z', { pollMs: 1, maxPolls: 5 });

		expect(ok).toBe(true);
		expect(mockToasterSuccess).toHaveBeenCalledWith(
			expect.objectContaining({ description: '7 photos in your album' })
		);
	});

	it('uses the singular noun for a one-photo album', async () => {
		mockSyncLibrary.mockResolvedValue({ error: undefined });
		mockGetLibrary.mockResolvedValue({
			data: { backend: 'immich', sync: { last_sync: '2026-06-05T10:00:00Z', asset_count: 1 } },
			error: undefined
		});

		await syncLibrary('2026-06-05T09:00:00Z', fast);

		expect(mockToasterSuccess).toHaveBeenCalledWith(
			expect.objectContaining({ description: '1 photo in your album' })
		);
	});
});
