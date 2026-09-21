import { describe, it, expect, vi, afterEach } from 'vitest';
import { invalidate } from '$app/navigation';
import { toaster } from './toaster';
import { loadImages, deleteImage, deleteImages, uploadImage, uploadImages } from './images';

vi.mock('$app/navigation', () => ({
	invalidate: vi.fn().mockResolvedValue(undefined)
}));

vi.mock('./toaster', () => ({
	toaster: { error: vi.fn(), success: vi.fn() }
}));

const mockFileToJpegBlob = vi.fn();

vi.mock('./imageProcessing', () => ({
	fileToJpegBlob: (...args: unknown[]) => mockFileToJpegBlob(...args)
}));

function jpegs(...names: string[]): File[] {
	return names.map((name) => new File(['x'], name, { type: 'image/jpeg' }));
}

const mockListImages = vi.fn();
const mockDeleteImage = vi.fn();
const mockUploadImage = vi.fn();
const mockSetImageOrder = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiListImages: (...args: unknown[]) => mockListImages(...args),
	apiDeleteImage: (...args: unknown[]) => mockDeleteImage(...args),
	apiUploadImage: (...args: unknown[]) => mockUploadImage(...args),
	apiSetImageOrder: (...args: unknown[]) => mockSetImageOrder(...args)
}));

describe('images', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('loadImages', () => {
		it('returns the parsed payload on ok response', async () => {
			const list = [{ name: 'a.jpg' }, { name: 'b.jpg' }];
			mockListImages.mockResolvedValue({ data: list, error: undefined });

			const result = await loadImages(fetch);
			expect(result).toEqual(list);
		});

		it('passes fetch to the SDK call', async () => {
			const customFetch = vi.fn();
			mockListImages.mockResolvedValue({ data: [], error: undefined });

			await loadImages(customFetch);

			expect(mockListImages).toHaveBeenCalledWith(expect.objectContaining({ fetch: customFetch }));
		});

		it('returns empty array when data is undefined', async () => {
			mockListImages.mockResolvedValue({ data: undefined, error: undefined });

			const result = await loadImages(fetch);
			expect(result).toEqual([]);
		});

		it('throws when the request fails', async () => {
			mockListImages.mockResolvedValue({ data: undefined, error: { status: 500 } });

			await expect(loadImages(fetch)).rejects.toThrow('Failed to load images');
		});
	});

	describe('deleteImage', () => {
		it('sends the image name in path param', async () => {
			mockDeleteImage.mockResolvedValue({ error: undefined });

			await deleteImage('cat.jpg');

			expect(mockDeleteImage).toHaveBeenCalledWith(
				expect.objectContaining({ path: { name: 'cat.jpg' } })
			);
		});

		it('invalidates /api/images on success', async () => {
			mockDeleteImage.mockResolvedValue({ error: undefined });

			await deleteImage('cat.jpg');

			expect(invalidate).toHaveBeenCalledWith('/api/images');
			expect(toaster.error).not.toHaveBeenCalled();
		});

		it('toasts and does not invalidate on error', async () => {
			mockDeleteImage.mockResolvedValue({ error: { status: 404 } });

			await deleteImage('cat.jpg');

			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Could not delete image',
				description: 'Server returned an error'
			});
			expect(invalidate).not.toHaveBeenCalled();
		});
	});

	describe('deleteImages', () => {
		it('deletes every name and invalidates only once', async () => {
			mockDeleteImage.mockResolvedValue({ error: undefined });

			await deleteImages(['a.jpg', 'b.jpg', 'c.jpg']);

			expect(mockDeleteImage).toHaveBeenCalledTimes(3);
			expect(mockDeleteImage).toHaveBeenNthCalledWith(1, { path: { name: 'a.jpg' } });
			expect(mockDeleteImage).toHaveBeenNthCalledWith(3, { path: { name: 'c.jpg' } });
			expect(invalidate).toHaveBeenCalledTimes(1);
			expect(invalidate).toHaveBeenCalledWith('/api/images');
			expect(toaster.error).not.toHaveBeenCalled();
		});

		it('aggregates failures into one toast but still refreshes the successes', async () => {
			mockDeleteImage
				.mockResolvedValueOnce({ error: undefined })
				.mockResolvedValueOnce({ error: { status: 500 } });

			await deleteImages(['ok.jpg', 'bad.jpg']);

			expect(toaster.error).toHaveBeenCalledTimes(1);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Could not delete 1 image',
				description: 'Server returned an error'
			});
			expect(invalidate).toHaveBeenCalledTimes(1);
		});

		it('does not invalidate when every delete fails', async () => {
			mockDeleteImage.mockResolvedValue({ error: { status: 500 } });

			await deleteImages(['x.jpg', 'y.jpg']);

			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Could not delete 2 images',
				description: 'Server returned an error'
			});
			expect(invalidate).not.toHaveBeenCalled();
		});
	});

	describe('uploadImage', () => {
		it('sends the blob in body', async () => {
			mockUploadImage.mockResolvedValue({ error: undefined });
			const blob = new Blob(['x'], { type: 'image/jpeg' });

			await uploadImage(blob);

			expect(mockUploadImage).toHaveBeenCalledWith(
				expect.objectContaining({ body: { image: blob } })
			);
		});

		it('invalidates /api/images and returns true on success', async () => {
			mockUploadImage.mockResolvedValue({ error: undefined });

			const result = await uploadImage(new Blob());

			expect(result).toBe(true);
			expect(invalidate).toHaveBeenCalledWith('/api/images');
			expect(toaster.error).not.toHaveBeenCalled();
		});

		it('toasts and returns false on error', async () => {
			mockUploadImage.mockResolvedValue({ error: { status: 503 } });

			const result = await uploadImage(new Blob());

			expect(result).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Could not upload image',
				description: 'Server returned an error'
			});
			expect(invalidate).not.toHaveBeenCalled();
		});
	});

	describe('uploadImages', () => {
		it('uploads every file and invalidates only once', async () => {
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockResolvedValue({ error: undefined });

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg'));

			expect(mockUploadImage).toHaveBeenCalledTimes(3);
			expect(mockUploadImage).toHaveBeenCalledWith(
				expect.objectContaining({ body: { image: expect.any(Blob) } })
			);
			expect(invalidate).toHaveBeenCalledExactlyOnceWith('/api/images');
			expect(result.added).toBe(3);
		});

		it('skips a file that cannot be decoded and keeps going', async () => {
			mockFileToJpegBlob.mockImplementation((file: File) =>
				file.name === 'b.jpg' ? Promise.reject(new Error('bad')) : Promise.resolve(new Blob())
			);
			mockUploadImage.mockResolvedValue({ error: undefined });

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg'));

			expect(mockUploadImage).toHaveBeenCalledTimes(2);
			expect(result.added).toBe(2);
			expect(result.failed).toEqual(['b.jpg']);
		});

		it('gives up after three uploads in a row fail', async () => {
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockResolvedValue({ error: new Error('offline') });

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg', 'd.jpg', 'e.jpg'));

			expect(mockUploadImage).toHaveBeenCalledTimes(3);
			expect(result.outcome).toBe('unreachable');
		});

		it('does not let photos the frame refuses trip the give-up counter', async () => {
			// A run of refusals must not read as the frame going away.
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockResolvedValue({
				error: new Error('unsupported'),
				response: new Response('', { status: 415 })
			});

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg', 'd.jpg'));

			expect(result.outcome).toBe('complete');
			expect(result.failed).toHaveLength(4);
		});

		it('still reports a result when the gallery refresh fails', async () => {
			// A throw would leave the caller's progress panel up for good.
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockResolvedValue({ error: undefined });
			vi.mocked(invalidate).mockRejectedValueOnce(new Error('navigation failed'));

			const result = await uploadImages(jpegs('a.jpg'));

			expect(result.added).toBe(1);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Photos added, but the list did not refresh',
				description: 'Reload the page to see them.'
			});
		});

		it('does not let unreadable files trip the give-up counter', async () => {
			// Unreadable files cluster (a run of HEICs off one phone), so counting
			// them would abort a batch the frame is happily accepting.
			mockFileToJpegBlob.mockImplementation((file: File) =>
				file.name === 'ok.jpg' ? Promise.resolve(new Blob()) : Promise.reject(new Error('bad'))
			);
			mockUploadImage.mockResolvedValue({ error: undefined });

			const result = await uploadImages(jpegs('1.heic', '2.heic', '3.heic', '4.heic', 'ok.jpg'));

			expect(result.outcome).toBe('complete');
			expect(result.added).toBe(1);
		});

		it('reports progress after every file, readable and unreadable alike', async () => {
			mockFileToJpegBlob.mockImplementation((file: File) =>
				file.name === 'b.jpg' ? Promise.reject(new Error('bad')) : Promise.resolve(new Blob())
			);
			mockUploadImage.mockResolvedValue({ error: undefined });
			const seen: number[] = [];

			await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg'), {
				onProgress: (done, total) => {
					expect(total).toBe(3);
					seen.push(done);
				}
			});

			expect(seen).toEqual([1, 2, 3]);
		});

		it('stops when the caller aborts', async () => {
			const controller = new AbortController();
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockImplementation(() => {
				controller.abort();
				return Promise.resolve({ error: undefined });
			});

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg'), {
				signal: controller.signal
			});

			expect(mockUploadImage).toHaveBeenCalledTimes(1);
			expect(result.outcome).toBe('stopped');
			// The frame stored this one before the stop landed, so it counts.
			expect(result.added).toBe(1);
		});

		it('gives up when the frame itself keeps erroring', async () => {
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockResolvedValue({
				error: new Error('boom'),
				response: new Response('', { status: 500 })
			});

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg', 'd.jpg'));

			expect(mockUploadImage).toHaveBeenCalledTimes(3);
			expect(result.outcome).toBe('unreachable');
		});

		it('treats the lowest refusal status as a refusal, not a broken frame', async () => {
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockResolvedValue({
				error: new Error('bad request'),
				response: new Response('', { status: 400 })
			});

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg', 'd.jpg'));

			expect(mockUploadImage).toHaveBeenCalledTimes(4);
			expect(result.outcome).toBe('complete');
		});

		it('hands the abort signal to the request, so a stalled upload can be cut off', async () => {
			const controller = new AbortController();
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockResolvedValue({ error: undefined });

			await uploadImages(jpegs('a.jpg'), { signal: controller.signal });

			expect(mockUploadImage).toHaveBeenCalledWith(
				expect.objectContaining({ signal: controller.signal })
			);
		});

		it('does not blame a photo that was interrupted mid-upload', async () => {
			const controller = new AbortController();
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockImplementation(() => {
				controller.abort();
				return Promise.reject(new DOMException('aborted', 'AbortError'));
			});

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg'), { signal: controller.signal });

			expect(result.outcome).toBe('stopped');
			expect(result.failed).toEqual([]);
		});

		it('counts a thrown upload as a failure when there is no signal to blame', async () => {
			mockFileToJpegBlob.mockResolvedValue(new Blob());
			mockUploadImage.mockRejectedValue(new Error('socket hung up'));

			const result = await uploadImages(jpegs('a.jpg', 'b.jpg', 'c.jpg', 'd.jpg'));

			expect(mockUploadImage).toHaveBeenCalledTimes(3);
			expect(result.outcome).toBe('unreachable');
		});

		it('does not invalidate when nothing was added', async () => {
			mockFileToJpegBlob.mockRejectedValue(new Error('bad'));

			const result = await uploadImages(jpegs('a.heic', 'b.heic'));

			expect(invalidate).not.toHaveBeenCalled();
			expect(result.failed).toEqual(['a.heic', 'b.heic']);
		});
	});

	describe('setImageOrder', () => {
		it('returns true and sends names with commit=false by default', async () => {
			mockSetImageOrder.mockResolvedValue({ error: undefined });
			const { setImageOrder } = await import('./images');
			const ok = await setImageOrder(['b.jpg', 'a.jpg']);
			expect(ok).toBe(true);
			expect(mockSetImageOrder).toHaveBeenCalledWith({
				body: { names: ['b.jpg', 'a.jpg'], commit: false }
			});
			expect(toaster.error).not.toHaveBeenCalled();
		});

		it('sends commit=true when committing', async () => {
			mockSetImageOrder.mockResolvedValue({ error: undefined });
			const { setImageOrder } = await import('./images');
			await setImageOrder(['a.jpg'], true);
			expect(mockSetImageOrder).toHaveBeenCalledWith({ body: { names: ['a.jpg'], commit: true } });
		});

		it('returns false and toasts on error', async () => {
			mockSetImageOrder.mockResolvedValue({ error: { message: 'boom' } });
			const { setImageOrder } = await import('./images');
			const ok = await setImageOrder(['a.jpg']);
			expect(ok).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Could not save photo order',
				description: 'Server returned an error'
			});
		});
	});
});
