import { invalidate } from '$app/navigation';
import { apiListImages, apiDeleteImage, apiUploadImage, apiSetImageOrder } from '$lib/api/sdk.gen';
import type { ListImagesData } from '$lib/api/types.gen';
import { toaster } from './toaster';
import { fileToJpegBlob } from './imageProcessing';

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

// Upload failures in a row that mean the frame is gone, not one awkward photo.
const giveUpAfterFailures = 3;

export type BulkUploadOutcome = 'complete' | 'stopped' | 'unreachable';

export interface BulkUploadResult {
	added: number;
	failed: string[];
	outcome: BulkUploadOutcome;
}

export interface BulkUploadOptions {
	onProgress?: (done: number, total: number) => void;
	signal?: AbortSignal;
}

// 'refused' is a 4xx verdict on this photo; 'rejected' means the frame itself did
// not answer, which is the only thing the give-up counter watches.
type FileOutcome = 'added' | 'unreadable' | 'refused' | 'rejected' | 'aborted';

// No response at all is a network error, per the generated client's contract.
function failureKind(response: Response | undefined): FileOutcome {
	if (response && response.status >= 400 && response.status < 500) return 'refused';
	return 'rejected';
}

async function uploadOne(file: File, signal?: AbortSignal): Promise<FileOutcome> {
	let blob: Blob;
	try {
		blob = await fileToJpegBlob(file);
	} catch {
		return 'unreadable';
	}
	// The signal goes to the request, not just the loop: a stalled upload never
	// resolves, so the loop would never reach its own abort check.
	try {
		const { error, response } = await apiUploadImage({ body: { image: blob }, signal });
		// A stop that lands mid-request does not undo a photo the frame already stored.
		if (!error) return 'added';
		return signal?.aborted ? 'aborted' : failureKind(response);
	} catch {
		return signal?.aborted ? 'aborted' : 'rejected';
	}
}

// Uploads files one at a time, skipping the cropper. Sequential because in parallel
// the browser would hold every full-size bitmap at once.
export async function uploadImages(
	files: File[],
	options: BulkUploadOptions = {}
): Promise<BulkUploadResult> {
	const { onProgress, signal } = options;
	let added = 0;
	let done = 0;
	let consecutiveFailures = 0;
	const failed: string[] = [];
	let outcome: BulkUploadOutcome = 'complete';

	for (const file of files) {
		if (signal?.aborted) {
			outcome = 'stopped';
			break;
		}
		const status = await uploadOne(file, signal);
		// Breaks before the counters: an interrupted photo is not an attempt.
		if (status === 'aborted') {
			outcome = 'stopped';
			break;
		}
		done++;
		onProgress?.(done, files.length);

		if (status === 'added') {
			added++;
			consecutiveFailures = 0;
			continue;
		}
		failed.push(file.name);
		if (status === 'unreadable' || status === 'refused') {
			consecutiveFailures = 0;
			continue;
		}
		consecutiveFailures++;
		if (consecutiveFailures >= giveUpAfterFailures) {
			outcome = 'unreachable';
			break;
		}
	}

	// Never rejects: a throw here would strand the caller's progress panel on screen.
	if (added > 0) {
		try {
			await invalidate('/api/images' satisfies ListImagesData['url']);
		} catch {
			toaster.error({
				title: 'Photos added, but the list did not refresh',
				description: 'Reload the page to see them.'
			});
		}
	}
	return { added, failed, outcome };
}

export async function setImageOrder(names: string[], commit = false): Promise<boolean> {
	const { error } = await apiSetImageOrder({ body: { names, commit } });
	if (!error) return true;
	toaster.error({
		title: 'Could not save photo order',
		description: 'Server returned an error'
	});
	return false;
}
