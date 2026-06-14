import { apiGetSystemDevices } from '$lib/api/sdk.gen';
import type { SystemDevicesBody } from '$lib/api/types.gen';

export type { SystemDevicesBody };

// loadDevices enumerates the hardware that seeds the settings device selects.
// Failure is non-fatal: the selects fall back to allow-custom free text, so a
// null result simply means "no suggestions", never a broken page.
export async function loadDevices(
	fetch: typeof globalThis.fetch
): Promise<SystemDevicesBody | null> {
	try {
		const { data, error } = await apiGetSystemDevices({ fetch });
		if (error) return null;
		return data ?? null;
	} catch {
		return null;
	}
}
