import { loadWiFiStatus } from '$lib/wifi';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch }) => {
	const status = await loadWiFiStatus(fetch);
	return { status };
};
