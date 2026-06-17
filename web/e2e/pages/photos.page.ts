import type { Locator, Page } from '@playwright/test';

export class PhotosPage {
	readonly uploadInput: Locator;
	readonly cropperUpload: Locator;
	readonly cropperUploadOriginal: Locator;
	readonly thumbs: Locator;
	readonly onScreen: Locator;
	readonly selectButton: Locator;
	readonly selectCancel: Locator;
	readonly bulkDelete: Locator;
	readonly bulkConfirm: Locator;
	readonly deleteConfirm: Locator;
	readonly lightbox: Locator;
	readonly lightboxClose: Locator;
	readonly immichStatus: Locator;
	readonly immichSync: Locator;

	constructor(private readonly page: Page) {
		this.uploadInput = page.getByTestId('photo-upload-input');
		this.cropperUpload = page.getByTestId('cropper-upload');
		this.cropperUploadOriginal = page.getByTestId('cropper-upload-original');
		this.thumbs = page.getByTestId('photo-thumb');
		this.onScreen = page.getByTestId('photo-onscreen');
		this.selectButton = page.getByTestId('photos-select');
		this.selectCancel = page.getByTestId('photos-select-cancel');
		this.bulkDelete = page.getByTestId('photos-bulk-delete');
		this.bulkConfirm = page.getByTestId('photos-bulk-confirm');
		this.deleteConfirm = page.getByTestId('photo-delete-confirm');
		this.lightbox = page.getByTestId('lightbox');
		this.lightboxClose = page.getByTestId('lightbox-close');
		this.immichStatus = page.getByTestId('immich-status');
		this.immichSync = page.getByTestId('immich-sync');
	}

	card(name: string): Locator {
		return this.page.getByTestId(`photo-card-${name}`);
	}

	ratioChip(id: string): Locator {
		return this.page.getByTestId(`crop-ratio-${id}`);
	}

	/** Thumbnail srcs, to diff an upload. */
	thumbSrcs(): Promise<string[]> {
		return this.thumbs.evaluateAll((els) => els.map((e) => e.getAttribute('src') ?? ''));
	}

	deleteButton(name: string): Locator {
		return this.page.getByTestId(`photo-delete-${name}`);
	}

	async goto(): Promise<void> {
		await this.page.goto('/admin/images');
	}

	async deletePhoto(name: string): Promise<void> {
		await this.card(name).hover();
		await this.deleteButton(name).click();
		await this.deleteConfirm.click();
	}
}
