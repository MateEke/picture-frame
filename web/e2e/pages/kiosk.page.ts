import type { Locator, Page } from '@playwright/test';
import { expect } from '@playwright/test';

export class KioskPage {
	readonly imgBottom: Locator;
	readonly overlay: Locator;
	readonly clock: Locator;
	readonly clockBlock: Locator;
	readonly readings: Locator;
	readonly date: Locator;
	readonly tempInside: Locator;
	readonly tempOutside: Locator;
	readonly labelInside: Locator;
	readonly labelOutside: Locator;
	readonly labelHumidity: Locator;
	readonly weatherIcon: Locator;
	readonly touchNav: Locator;
	readonly menu: Locator;
	readonly slideshow: Locator;

	constructor(private readonly page: Page) {
		this.imgBottom = page.getByTestId('kiosk-img-bottom');
		this.overlay = page.getByTestId('kiosk-overlay');
		this.clock = page.getByTestId('kiosk-clock');
		this.clockBlock = page.getByTestId('kiosk-clock-block');
		this.readings = page.getByTestId('kiosk-readings');
		this.date = page.getByTestId('kiosk-date');
		this.tempInside = page.getByTestId('kiosk-temp-inside');
		this.tempOutside = page.getByTestId('kiosk-temp-outside');
		this.labelInside = page.getByTestId('kiosk-label-inside');
		this.labelOutside = page.getByTestId('kiosk-label-outside');
		this.labelHumidity = page.getByTestId('kiosk-label-humidity');
		this.weatherIcon = page.getByTestId('kiosk-weather-icon');
		this.touchNav = page.getByTestId('kiosk-touch-nav');
		this.menu = page.getByTestId('touch-menu');
		this.slideshow = page.getByTestId('touch-slideshow');
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

	/** Resolves once the page's SSE state reflects the panel being off. */
	async waitForScreenOff(): Promise<void> {
		await expect(this.touchNav).toHaveAttribute('data-screen-off', 'true');
	}

	/** Clicks at the given fractions across and down the viewport. */
	async tapAt(xFraction: number, yFraction: number): Promise<void> {
		const size = this.page.viewportSize();
		if (!size) throw new Error('no viewport size');
		await this.page.mouse.click(size.width * xFraction, size.height * yFraction);
	}

	/** A horizontal drag across the middle of the screen; left = next photo. */
	async swipe(direction: 'left' | 'right'): Promise<void> {
		const size = this.page.viewportSize();
		if (!size) throw new Error('no viewport size');
		const y = size.height / 2;
		const [from, to] = direction === 'left' ? [0.8, 0.2] : [0.2, 0.8];
		await this.page.mouse.move(size.width * from, y);
		await this.page.mouse.down();
		await this.page.mouse.move(size.width * to, y, { steps: 5 });
		await this.page.mouse.up();
	}

	/** Opens the touch menu from the slideshow with a tap. */
	async openMenu(): Promise<void> {
		await this.tapAt(0.5, 0.5);
		await expect(this.menu).toBeVisible();
	}

	tab(id: 'home' | 'gallery' | 'upload' | 'files' | 'settings'): Locator {
		return this.page.getByTestId(`touch-tab-${id}`);
	}

	/** Waits for the settled bottom-layer src to differ from `from`. */
	async waitForImageChange(from: string, timeoutMs = 10_000): Promise<string> {
		await expect(this.imgBottom).not.toHaveAttribute('src', from, { timeout: timeoutMs });
		return String(await this.currentImageSrc());
	}

	/** The element's translate, in CSS px. */
	shiftOf(target: Locator): Promise<{ x: number; y: number }> {
		return target.evaluate((el) => {
			const t = getComputedStyle(el).transform;
			const m = new DOMMatrixReadOnly(t === 'none' ? '' : t);
			return { x: m.m41, y: m.m42 };
		});
	}

	rootFontSize(): Promise<number> {
		return this.page.evaluate(() =>
			parseFloat(getComputedStyle(document.documentElement).fontSize)
		);
	}
}
