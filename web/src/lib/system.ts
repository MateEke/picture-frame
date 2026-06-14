import { apiGetSystemInfo } from '$lib/api/sdk.gen';
import type { SystemInfoBody } from '$lib/api/types.gen';

export type { SystemInfoBody };

// loadSystemInfo fetches version/uptime/hostname/ip for the dashboard system card.
// Failure is non-fatal: the card just hides the fields it has no data for.
export async function loadSystemInfo(
	fetch: typeof globalThis.fetch
): Promise<SystemInfoBody | null> {
	try {
		const { data, error } = await apiGetSystemInfo({ fetch });
		if (error) return null;
		return data ?? null;
	} catch {
		return null;
	}
}
