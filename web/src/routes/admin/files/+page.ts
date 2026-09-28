import { loadFiles } from '$lib/files';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, depends }) => {
	depends('app:files');
	try {
		return { ...(await loadFiles(fetch)), error: null };
	} catch (err) {
		return { files: null, freeBytes: null, error: err instanceof Error ? err.message : 'unknown' };
	}
};
