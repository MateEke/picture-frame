import path from 'node:path';
import { expect, test } from './fixtures';
import { seedImagesDir } from './helpers';

const UPLOAD = path.join(seedImagesDir(), 'red.jpg');

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

	test('uploads a photo through the cropper', async ({ photos }) => {
		await photos.uploadInput.setInputFiles(UPLOAD);
		await photos.cropperUpload.click();
		await expect(photos.thumbs).toHaveCount(4);
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
