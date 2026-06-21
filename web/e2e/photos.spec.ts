import path from 'node:path';
import type { Page } from '@playwright/test';
import { expect, test } from './fixtures';
import { jpegSize, seedImagesDir } from './helpers';
import type { PhotosPage } from './pages/photos.page';

// A square source (seeds are landscape) so the no-crop test can assert it stays square.
const UPLOAD = path.join(seedImagesDir(), '..', 'square.jpg');
// Landscape pixels with an EXIF rotate-90 tag (displayed portrait).
const ROTATED = path.join(seedImagesDir(), '..', 'rotated.jpg');

// Size of the stored image that appeared since `before`.
async function uploadedSize(page: Page, photos: PhotosPage, before: string[]) {
	const added = (await photos.thumbSrcs()).find((s) => !before.includes(s));
	if (!added) throw new Error('no new image appeared after upload');
	const res = await page.request.get(added);
	return jpegSize(await res.body());
}

test.describe('photos', () => {
	test.beforeEach(async ({ photos }) => {
		await photos.goto();
		await expect(photos.thumbs.first()).toBeVisible();
	});

	test('lists the seeded photos', async ({ photos }) => {
		await expect(photos.thumbs).toHaveCount(3);
	});

	test('badges the photo currently on screen', async ({ photos }) => {
		await expect(photos.onScreen).toBeVisible();
	});

	test('crops the upload to the chosen ratio', async ({ photos, page }) => {
		await photos.uploadInput.setInputFiles(UPLOAD);
		const before = await photos.thumbSrcs();
		await photos.cropperUpload.click(); // default 16:9
		await expect(photos.thumbs).toHaveCount(4);
		const { width, height } = await uploadedSize(page, photos, before);
		expect(width / height).toBeCloseTo(16 / 9, 1);
	});

	test('uploads without cropping at the original aspect ratio', async ({ photos, page }) => {
		await photos.uploadInput.setInputFiles(UPLOAD);
		const before = await photos.thumbSrcs();
		await photos.cropperUploadOriginal.click();
		await expect(photos.thumbs).toHaveCount(4);
		// Uncropped: the square seed stays square.
		const { width, height } = await uploadedSize(page, photos, before);
		expect(width).toBe(height);
	});

	test('bakes EXIF orientation on no-crop upload', async ({ photos, page }) => {
		// rotated.jpg is landscape pixels with an EXIF rotate-90 tag; the no-crop path
		// must bake it upright, else split-screen mis-reads it as landscape and crops it.
		await photos.uploadInput.setInputFiles(ROTATED);
		const before = await photos.thumbSrcs();
		await photos.cropperUploadOriginal.click();
		await expect(photos.thumbs).toHaveCount(4);
		const { width, height } = await uploadedSize(page, photos, before);
		expect(height).toBeGreaterThan(width);
	});

	test('remembers the chosen crop ratio across reloads', async ({ photos, page }) => {
		await photos.uploadInput.setInputFiles(UPLOAD);
		await photos.ratioChip('4:3').click();
		await expect(photos.ratioChip('4:3')).toHaveAttribute('aria-pressed', 'true');

		await page.reload();
		await expect(photos.thumbs.first()).toBeVisible();
		await photos.uploadInput.setInputFiles(UPLOAD);
		await expect(photos.ratioChip('4:3')).toHaveAttribute('aria-pressed', 'true');
		await expect(photos.ratioChip('16:9')).toHaveAttribute('aria-pressed', 'false');
	});

	test('opens and closes the lightbox', async ({ photos }) => {
		await photos.card('red.jpg').click();
		await expect(photos.lightbox).toBeVisible();
		await photos.lightboxClose.click();
		await expect(photos.lightbox).toBeHidden();
	});

	test('deletes a photo from the grid on hover', async ({ photos }, testInfo) => {
		// Per-card trash is hover-only; mobile deletes via bulk-delete instead.
		test.skip(testInfo.project.name === 'mobile-chromium', 'grid hover-delete is desktop-only');
		await photos.deletePhoto('red.jpg');
		await expect(photos.card('red.jpg')).toHaveCount(0);
		await expect(photos.thumbs).toHaveCount(2);
	});

	test('bulk-deletes selected photos', async ({ photos }) => {
		await photos.selectButton.click();
		await photos.card('red.jpg').click();
		await photos.card('green.jpg').click();
		await photos.bulkDelete.click();
		await photos.bulkConfirm.click();
		await expect(photos.thumbs).toHaveCount(1);
	});

	test('cancel leaves select mode', async ({ photos }) => {
		await photos.selectButton.click();
		await photos.selectCancel.click();
		await expect(photos.selectButton).toBeVisible();
	});

	test('arrange mode shows move controls and hides per-tile delete', async ({ photos }) => {
		await photos.arrangeButton.click();
		// Move buttons appear on the first seeded card.
		await expect(photos.moveDown('red.jpg')).toBeVisible();
		// Delete buttons are not rendered in arrange mode.
		await expect(photos.deleteButton('red.jpg')).toHaveCount(0);
		await photos.arrangeDone.click();
		await expect(photos.arrangeButton).toBeVisible();
	});

	test('reorders photos and persists across reload', async ({ photos, page }) => {
		await photos.arrangeButton.click();
		const before = await photos.cardOrder();
		await photos.moveDown(before[0]).click();
		await expect.poll(() => photos.cardOrder()).not.toEqual(before);
		const after = await photos.cardOrder();
		// Done flushes the pending save before we reload.
		await photos.arrangeDone.click();
		await page.reload();
		await expect(photos.thumbs.first()).toBeVisible();
		await expect.poll(() => photos.cardOrder()).toEqual(after);
	});

	test('Done commits the order to the server', async ({ photos, page }) => {
		const commits: string[] = [];
		page.on('request', (r) => {
			if (r.method() === 'PUT' && r.url().endsWith('/api/images/order')) {
				commits.push(r.postData() ?? '');
			}
		});
		await photos.arrangeButton.click();
		const before = await photos.cardOrder();
		await photos.moveDown(before[0]).click();
		await expect.poll(() => photos.cardOrder()).not.toEqual(before);
		await photos.arrangeDone.click();
		await expect(photos.arrangeButton).toBeVisible();
		expect(commits.some((b) => b.includes('"commit":true'))).toBe(true);
	});
});

// Separate group: the immich grid is empty, so the fs beforeEach doesn't apply.
test.describe('photos (immich backend)', () => {
	test.use({ serverOptions: { immich: true } });

	test('shows a read-only immich view', async ({ page, photos }) => {
		await page.goto('/admin/images');
		await expect(photos.immichStatus).toBeVisible();
		await expect(photos.immichSync).toBeVisible();
		await expect(photos.uploadInput).toHaveCount(0);
		await expect(photos.selectButton).toHaveCount(0);
	});
});
