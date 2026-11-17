import type { PageLoad } from './$types';
import { loadConfig, loadConfigMeta } from '#lib/config.js';
import { loadDevices } from '#lib/devices.js';
import { loadUpdate } from '#lib/updater.js';

export const load: PageLoad = async ({ fetch }) => {
	const [config, meta, devices, update] = await Promise.all([
		loadConfig(fetch),
		loadConfigMeta(fetch),
		loadDevices(fetch),
		loadUpdate(fetch)
	]);
	return { config, meta, devices, update };
};
