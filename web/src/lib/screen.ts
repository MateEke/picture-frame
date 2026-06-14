import { apiGetScreen, apiSetScreen } from '$lib/api/sdk.gen';
import { toaster } from './toaster';

export async function loadScreen(fetch: typeof globalThis.fetch) {
	const { data, error } = await apiGetScreen({ fetch });
	if (error) throw new Error('Failed to load screen state');
	return data;
}

// No invalidate: consumers reflect the new state from the screen SSE event, so there's
// no need to refetch (which would rerun the whole page load).
export async function setScreen(on: boolean): Promise<void> {
	const { error } = await apiSetScreen({ body: { state: on ? 'on' : 'off' } });
	if (error) {
		toaster.error({ title: 'Could not toggle screen', description: 'Server returned an error' });
	}
}
