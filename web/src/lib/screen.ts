import { apiGetScreen, apiSetScreen } from '#lib/api/sdk.gen.js';
import type { NextChangeDto } from '#lib/api/types.gen.js';
import { nextChangeTime } from './schedule';
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

// True whoever turned the screen off: a manual off still ends at the scheduled on.
export function screenReason(
	auto: boolean,
	on: boolean,
	next: NextChangeDto | null | undefined,
	locale: string,
	now: number
): string {
	if (!auto) {
		return next?.on
			? `Off. Turns back on at ${nextChangeTime(next, locale, now)}.`
			: 'Turned off manually.';
	}
	if (!on) return 'Asleep. Wakes on motion or activity.';
	return next && !next.on
		? `Showing photos. Turns off at ${nextChangeTime(next, locale, now)}.`
		: 'Showing photos.';
}
