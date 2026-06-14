import { invalidate } from '$app/navigation';
import { apiGetLibrary, apiSyncLibrary } from '$lib/api/sdk.gen';
import type { LibrarySync } from '$lib/api/types.gen';
import { toaster } from './toaster';

export async function loadLibrary(fetch: typeof globalThis.fetch) {
	const { data, error } = await apiGetLibrary({ fetch });
	if (error) throw new Error('Failed to load library info');
	return data;
}

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

// Polls until the syncer records an attempt newer than prevLastSync (which
// advances on success or failure), or the budget runs out (returns null).
async function waitForSyncResult(
	prevLastSync: string | undefined,
	pollMs: number,
	maxPolls: number
): Promise<LibrarySync | null> {
	for (let i = 0; i < maxPolls; i++) {
		await delay(pollMs);
		const { data } = await apiGetLibrary();
		const sync = data?.sync;
		if (sync?.last_sync && sync.last_sync !== prevLastSync) return sync;
	}
	return null;
}

// Triggers a remote sync and waits for it to finish before refreshing, so the new
// photos are on screen when the spinner stops. Returns false on a failed trigger
// or a sync that ended in error.
export async function syncLibrary(
	prevLastSync?: string,
	{ pollMs = 1500, maxPolls = 40 }: { pollMs?: number; maxPolls?: number } = {}
): Promise<boolean> {
	const { error } = await apiSyncLibrary();
	if (error) {
		toaster.error({ title: 'Could not start sync', description: 'Server returned an error' });
		return false;
	}

	const result = await waitForSyncResult(prevLastSync, pollMs, maxPolls);
	await Promise.all([invalidate('/api/library'), invalidate('/api/images')]);

	if (!result) {
		// Outran the budget; the 30s poll will catch it.
		toaster.success({ title: 'Sync started', description: 'New photos will appear shortly.' });
		return true;
	}
	if (result.last_error) {
		toaster.error({ title: 'Sync finished with errors', description: result.last_error });
		return false;
	}
	toaster.success({
		title: 'Sync complete',
		description: `${result.asset_count} ${result.asset_count === 1 ? 'photo' : 'photos'} in your album`
	});
	return true;
}
