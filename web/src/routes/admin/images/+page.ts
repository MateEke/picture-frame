import { loadImages } from '$lib/images';
import { loadLibrary } from '$lib/library';
import { loadConfig } from '$lib/config';
import type { PageLoad } from './$types';

function errorMessage(result: PromiseSettledResult<unknown>): string | undefined {
	if (result.status !== 'rejected') return undefined;
	if (result.reason instanceof Error) return result.reason.message;
	return 'unknown';
}

export const load: PageLoad = async ({ fetch }) => {
	const [imagesResult, libraryResult] = await Promise.allSettled([
		loadImages(fetch),
		loadLibrary(fetch)
	]);
	const images = imagesResult.status === 'fulfilled' ? imagesResult.value : null;
	// On library load failure, return null so the UI surfaces the error and
	// disables upload/delete instead of guessing a default backend.
	const library = libraryResult.status === 'fulfilled' ? libraryResult.value : null;
	// Only needed for the Immich share-URL link.
	const config = library?.backend === 'immich' ? await loadConfig(fetch) : null;
	return {
		images,
		imagesError: errorMessage(imagesResult),
		library,
		libraryError: errorMessage(libraryResult),
		shareUrl: config?.library.immich.share_url ?? null
	};
};
