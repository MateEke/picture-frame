import type { PageLoad } from './$types';
import { loadConfig, loadConfigMeta } from '$lib/config';
import { loadDevices } from '$lib/devices';
import { loadUpdate } from '$lib/updater';

export const load: PageLoad = async ({ fetch }) => {
	const [config, meta, devices, update] = await Promise.all([
		loadConfig(fetch),
		loadConfigMeta(fetch),
		loadDevices(fetch),
		loadUpdate(fetch)
	]);
	return { config, meta, devices, update };
};
