import { describe, it, expect, vi, afterEach } from 'vitest';
import { invalidate } from '$app/navigation';
import { toaster } from './toaster';
import { loadImages, deleteImage, deleteImages, uploadImage } from './images';

vi.mock('$app/navigation', () => ({
	invalidate: vi.fn().mockResolvedValue(undefined)
}));

vi.mock('./toaster', () => ({
	toaster: { error: vi.fn(), success: vi.fn() }
}));

const mockListImages = vi.fn();
const mockDeleteImage = vi.fn();
const mockUploadImage = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiListImages: (...args: unknown[]) => mockListImages(...args),
	apiDeleteImage: (...args: unknown[]) => mockDeleteImage(...args),
	apiUploadImage: (...args: unknown[]) => mockUploadImage(...args)
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
});
