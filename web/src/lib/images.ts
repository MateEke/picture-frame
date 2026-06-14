import { invalidate } from '$app/navigation';
import { apiListImages, apiDeleteImage, apiUploadImage } from '$lib/api/sdk.gen';
import type { ListImagesData } from '$lib/api/types.gen';
import { toaster } from './toaster';

export async function loadImages(fetch: typeof globalThis.fetch) {
	const { data, error } = await apiListImages({ fetch });
	if (error) throw new Error('Failed to load images');
	return data ?? [];
}

export async function deleteImage(name: string): Promise<void> {
	const { error } = await apiDeleteImage({ path: { name } });
	if (!error) {
		return invalidate('/api/images' satisfies ListImagesData['url']);
	}
	toaster.error({
		title: 'Could not delete image',
		description: 'Server returned an error'
	});
}

// One gallery invalidation for the whole batch.
export async function deleteImages(names: string[]): Promise<void> {
	let failed = 0;
	for (const name of names) {
		const { error } = await apiDeleteImage({ path: { name } });
		if (error) failed++;
	}
	if (failed > 0) {
		toaster.error({
			title: `Could not delete ${failed} ${failed === 1 ? 'image' : 'images'}`,
			description: 'Server returned an error'
		});
	}
	if (failed < names.length) {
		await invalidate('/api/images' satisfies ListImagesData['url']);
	}
}

export async function uploadImage(blob: Blob): Promise<boolean> {
	const { error } = await apiUploadImage({ body: { image: blob } });
	if (!error) {
		await invalidate('/api/images' satisfies ListImagesData['url']);
		return true;
	}
	toaster.error({
		title: 'Could not upload image',
		description: 'Server returned an error'
	});
	return false;
}
