import { apiDeleteFile, apiListFiles } from '$lib/api/sdk.gen';
import type { FileItem } from '$lib/api/types.gen';

export type { FileItem };

// Extensions the photo pipeline can turn into a slideshow JPEG when the browser
// decodes them. Anything else, or a photo the browser can't decode (e.g. HEIC
// outside Safari), is kept as a plain file instead of being dropped.
const photoExtensions = /\.(jpe?g|png|gif|webp|avif|heic|heif|bmp)$/i;

/** Whether a picked file should go to the slideshow rather than Files. */
export function isPhoto(file: Pick<File, 'name' | 'type'>): boolean {
	if (file.type) return file.type.startsWith('image/') && file.type !== 'image/svg+xml';
	return photoExtensions.test(file.name);
}

export function splitUploads<T extends Pick<File, 'name' | 'type'>>(
	files: T[]
): { photos: T[]; others: T[] } {
	const photos: T[] = [];
	const others: T[] = [];
	for (const f of files) (isPhoto(f) ? photos : others).push(f);
	return { photos, others };
}

export async function loadFiles(
	fetch: typeof globalThis.fetch
): Promise<{ files: FileItem[]; freeBytes: number | null }> {
	const { data, error } = await apiListFiles({ fetch });
	if (error || !data) throw new Error('Failed to load files');
	return { files: data.files ?? [], freeBytes: data.free_bytes ?? null };
}

export async function deleteFile(name: string): Promise<boolean> {
	const { error } = await apiDeleteFile({ path: { name } });
	return !error;
}

export type FileUploadOutcome = 'complete' | 'stopped' | 'unreachable';

export interface FileUploadResult {
	added: number;
	failed: string[];
	outcome: FileUploadOutcome;
}

export interface FileUploadOptions {
	/** Bytes sent across the whole batch, for one smooth progress bar. */
	onProgress?: (sentBytes: number, totalBytes: number, index: number) => void;
	signal?: AbortSignal;
	/** Injectable for tests. */
	send?: (file: File, onBytes: (n: number) => void, signal?: AbortSignal) => Promise<number>;
}

/**
 * Posts one file as multipart via XHR (fetch has no upload progress). Resolves
 * with the HTTP status; 0 means the request never got an answer.
 */
export function sendFile(
	file: File,
	onBytes: (n: number) => void,
	signal?: AbortSignal
): Promise<number> {
	return new Promise((resolve) => {
		const xhr = new XMLHttpRequest();
		xhr.open('POST', '/api/files');
		xhr.upload.onprogress = (e) => onBytes(e.loaded);
		xhr.onload = () => resolve(xhr.status);
		xhr.onerror = () => resolve(0);
		xhr.onabort = () => resolve(-1);
		signal?.addEventListener('abort', () => xhr.abort(), { once: true });
		const form = new FormData();
		form.append('file', file, file.name);
		xhr.send(form);
	});
}

// Answers in a row that mean the frame is gone, not one bad file.
const giveUpAfterFailures = 3;

/** Uploads one at a time; a phone on Wi-Fi gains nothing from parallel posts. */
export async function uploadFiles(
	files: File[],
	options: FileUploadOptions = {}
): Promise<FileUploadResult> {
	const { onProgress, signal, send = sendFile } = options;
	const total = files.reduce((n, f) => n + f.size, 0);
	let before = 0;
	let added = 0;
	let consecutive = 0;
	const failed: string[] = [];
	let outcome: FileUploadOutcome = 'complete';

	for (const [i, file] of files.entries()) {
		if (signal?.aborted) {
			outcome = 'stopped';
			break;
		}
		const status = await send(file, (n) => onProgress?.(before + n, total, i), signal);
		if (status === -1 || signal?.aborted) {
			outcome = 'stopped';
			break;
		}
		before += file.size;
		onProgress?.(before, total, i);
		if (status >= 200 && status < 300) {
			added++;
			consecutive = 0;
			continue;
		}
		failed.push(file.name);
		// A 4xx is about this file (too big, disk full); only silence counts toward giving up.
		if (status >= 400 && status < 500 && status !== 408) {
			consecutive = 0;
			continue;
		}
		consecutive++;
		if (consecutive >= giveUpAfterFailures) {
			outcome = 'unreachable';
			break;
		}
	}
	return { added, failed, outcome };
}
