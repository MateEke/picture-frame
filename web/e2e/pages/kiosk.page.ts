import type { Locator, Page } from '@playwright/test';
import { expect } from '@playwright/test';

export class KioskPage {
	readonly imgBottom: Locator;
	readonly overlay: Locator;
	readonly clock: Locator;
	readonly date: Locator;
	readonly tempInside: Locator;
	readonly tempOutside: Locator;
	readonly labelInside: Locator;
	readonly labelOutside: Locator;
	readonly labelHumidity: Locator;
	readonly weatherIcon: Locator;

	constructor(private readonly page: Page) {
		this.imgBottom = page.getByTestId('kiosk-img-bottom');
		this.overlay = page.getByTestId('kiosk-overlay');
		this.clock = page.getByTestId('kiosk-clock');
		this.date = page.getByTestId('kiosk-date');
		this.tempInside = page.getByTestId('kiosk-temp-inside');
		this.tempOutside = page.getByTestId('kiosk-temp-outside');
		this.labelInside = page.getByTestId('kiosk-label-inside');
		this.labelOutside = page.getByTestId('kiosk-label-outside');
		this.labelHumidity = page.getByTestId('kiosk-label-humidity');
		this.weatherIcon = page.getByTestId('kiosk-weather-icon');
	}

	async goto(): Promise<void> {
		await this.page.goto('/kiosk');
	}

	/** Resolves once the slideshow has published an image over SSE. */
	async waitForImage(): Promise<void> {
		await expect(this.imgBottom).toHaveAttribute('src', /^\/img\//);
	}

	currentImageSrc(): Promise<string | null> {
		return this.imgBottom.getAttribute('src');
	}

	/** Waits for the settled bottom-layer src to differ from `from`. */
	async waitForImageChange(from: string, timeoutMs = 10_000): Promise<string> {
		await expect(this.imgBottom).not.toHaveAttribute('src', from, { timeout: timeoutMs });
		return String(await this.currentImageSrc());
	}
}
