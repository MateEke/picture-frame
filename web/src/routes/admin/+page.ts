import { loadScreen } from '$lib/screen';
import { loadConfig } from '$lib/config';
import { loadWiFiStatus } from '$lib/wifi';
import { loadSystemInfo } from '$lib/system';
import { loadLibrary } from '$lib/library';
import { loadUpdate } from '$lib/updater';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	// Screen failure is surfaced inline; the rest degrade to "unavailable" tiles.
	const [config, wifi, system, library, update] = await Promise.all([
		loadConfig(fetch),
		loadWiFiStatus(fetch).catch(() => null),
		loadSystemInfo(fetch),
		loadLibrary(fetch).catch(() => null),
		loadUpdate(fetch)
	]);
	try {
		return { screen: await loadScreen(fetch), config, wifi, system, library, update };
	} catch (e) {
		return {
			screen: null,
			screenError: e instanceof Error ? e.message : 'unknown',
			config,
			wifi,
			system,
			library,
			update
		};
	}
};
