import { expect, test } from './fixtures';
import { splitImagesDir } from './helpers';

test.describe('kiosk', () => {
	test.beforeEach(async ({ kiosk }) => {
		await kiosk.goto();
		await kiosk.waitForImage();
	});

	test('advances to the next image (crossfade)', async ({ kiosk }) => {
		// Assert the settled bottom src; transient opacity is racy.
		const first = await kiosk.currentImageSrc();
		expect(first).not.toBeNull();
		const second = await kiosk.waitForImageChange(String(first));
		expect(second).not.toBe(String(first));
		expect(second).toMatch(/^\/img\//);
	});

	test('shows the clock and date', async ({ kiosk }) => {
		await expect(kiosk.clock).toContainText(/\d/);
		await expect(kiosk.date).toContainText(/[A-Za-z]/);
	});

	test('shows the configured cluster labels', async ({ kiosk }) => {
		await expect(kiosk.labelInside).toHaveText('E2E Inside');
		await expect(kiosk.labelOutside).toHaveText('E2E Outside');
		await expect(kiosk.labelHumidity).toHaveText('E2E Humidity');
	});

	test('shows live sensor temperatures', async ({ kiosk }) => {
		// Inside drifts, so assert numeric (not the "--" stale placeholder).
		await expect(kiosk.tempOutside).toContainText('5');
		await expect(kiosk.tempInside).toContainText(/\d/);
		await expect(kiosk.tempInside).not.toContainText('--');
	});

	test('shows the weather icon (mock active in dev)', async ({ kiosk }) => {
		await expect(kiosk.weatherIcon).toBeVisible();
		await expect(kiosk.weatherIcon).toHaveAttribute('src', /.+/);
	});
});

test.describe('kiosk overlay visibility', () => {
	test.describe('clock and date hidden', () => {
		test.use({ serverOptions: { hideClockDate: true } });

		test('hides the clock and date but keeps the readings', async ({ kiosk }) => {
			await kiosk.goto();
			await kiosk.waitForImage();
			await expect(kiosk.clock).toHaveCount(0);
			await expect(kiosk.date).toHaveCount(0);
			await expect(kiosk.overlay).toBeVisible();
			await expect(kiosk.tempInside).toBeVisible();
		});
	});

	test.describe('nothing configured', () => {
		test.use({ serverOptions: { minimalOverlay: true, hideClockDate: true } });

		test('hides the whole overlay', async ({ kiosk }) => {
			await kiosk.goto();
			await kiosk.waitForImage();
			await expect(kiosk.overlay).toHaveCount(0);
		});
	});

	test.describe('burn-in shift', () => {
		// UTC pins the minute of the hour; half-hour-offset zones would shift it.
		test.use({ timezoneId: 'UTC' });

		test('offsets the overlay content without moving the scrim', async ({ kiosk, page }) => {
			// Minute 24: both components non-zero. Readings may show "--" here (the pinned
			// clock can push them past SENSOR_STALE_MS); this asserts geometry only.
			const fixed = new Date();
			fixed.setUTCMinutes(24, 0, 0);
			await page.clock.setFixedTime(fixed);

			await kiosk.goto();
			await kiosk.waitForImage();

			const rem = await kiosk.rootFontSize();
			const clockShift = await kiosk.shiftOf(kiosk.clockBlock);

			// Past a third of the orbit: x negative, y positive.
			expect(clockShift.x).toBeLessThan(0);
			expect(clockShift.y).toBeGreaterThan(0);
			expect(Math.abs(clockShift.x)).toBeLessThanOrEqual(0.75 * rem);
			expect(Math.abs(clockShift.y)).toBeLessThanOrEqual(0.5 * rem);

			// Whole pixels, or a promoted layer would resample and soften the text.
			expect(Number.isInteger(clockShift.x)).toBe(true);
			expect(Number.isInteger(clockShift.y)).toBe(true);

			// Mirrored horizontally so both margins move together; level so the baseline holds.
			const readingsShift = await kiosk.shiftOf(kiosk.readings);
			expect(clockShift.x + readingsShift.x).toBe(0);
			expect(readingsShift.y).toBe(clockShift.y);

			const size = page.viewportSize();
			if (!size) throw new Error('no viewport size');
			const overlayBox = await kiosk.overlay.boundingBox();
			if (!overlayBox) throw new Error('no overlay box');
			expect(overlayBox.x).toBeCloseTo(0, 0);
			expect(overlayBox.width).toBeCloseTo(size.width, 0);
			expect(overlayBox.y + overlayBox.height).toBeCloseTo(size.height, 0);

			for (const block of [kiosk.clockBlock, kiosk.readings]) {
				const box = await block.boundingBox();
				if (!box) throw new Error('no block box');
				expect(box.x).toBeGreaterThan(0);
				expect(box.y).toBeGreaterThan(0);
				expect(box.x + box.width).toBeLessThan(size.width);
				expect(box.y + box.height).toBeLessThan(size.height);
			}
		});

		// Either block can be configured away, so each must shift on its own.
		test.describe('readings only', () => {
			test.use({ serverOptions: { hideClockDate: true } });

			test('still shifts the surviving block', async ({ kiosk, page }) => {
				const fixed = new Date();
				fixed.setUTCMinutes(24, 0, 0);
				await page.clock.setFixedTime(fixed);
				await kiosk.goto();
				await kiosk.waitForImage();

				await expect(kiosk.clockBlock).toHaveCount(0);
				const shift = await kiosk.shiftOf(kiosk.readings);
				expect(shift.x).toBeGreaterThan(0);
				expect(Number.isInteger(shift.x)).toBe(true);
				expect(Number.isInteger(shift.y)).toBe(true);
			});
		});

		test.describe('clock only', () => {
			test.use({ serverOptions: { minimalOverlay: true } });

			test('still shifts the surviving block', async ({ kiosk, page }) => {
				const fixed = new Date();
				fixed.setUTCMinutes(24, 0, 0);
				await page.clock.setFixedTime(fixed);
				await kiosk.goto();
				await kiosk.waitForImage();

				await expect(kiosk.readings).toHaveCount(0);
				const shift = await kiosk.shiftOf(kiosk.clockBlock);
				expect(shift.x).toBeLessThan(0);
				expect(Number.isInteger(shift.x)).toBe(true);
				expect(Number.isInteger(shift.y)).toBe(true);
			});
		});
	});

	test.describe('timezone', () => {
		test.use({ serverOptions: { timezone: 'Asia/Tokyo' } });

		test('formats the clock in the configured timezone', async ({ kiosk, page }) => {
			await kiosk.goto();
			await kiosk.waitForImage();
			await expect(kiosk.clock).toBeVisible();
			// The clock must read Tokyo wall time, not the runner's zone. Compare the exact
			// HH:MM digits (not substrings), re-evaluating per poll so a minute tick can't flake it.
			await expect
				.poll(async () => {
					const want = await page.evaluate(() => {
						const parts = new Intl.DateTimeFormat('en-US', {
							timeZone: 'Asia/Tokyo',
							hour: '2-digit',
							minute: '2-digit'
						}).formatToParts(new Date());
						const get = (t: string) => parts.find((p) => p.type === t)?.value ?? '';
						return `${get('hour')}${get('minute')}`;
					});
					const text = (await kiosk.clock.textContent()) ?? '';
					return text.replace(/\D/g, '') === want;
				})
				.toBe(true);
		});
	});
});

test.describe('kiosk split-screen', () => {
	// A portrait viewport makes the landscape seed photos outliers, so they pair.
	test.use({ viewport: { width: 640, height: 1000 } });

	test('pairs mismatched-orientation photos, stacked on a portrait screen', async ({
		kiosk,
		page
	}) => {
		await kiosk.goto();
		await kiosk.waitForImage();
		const panes = page.getByTestId('kiosk-slide-bottom').locator('> img');
		await expect.poll(() => panes.count(), { timeout: 15_000 }).toBe(2);

		const a = await panes.nth(0).boundingBox();
		const b = await panes.nth(1).boundingBox();
		// Stacked, not side-by-side: same column, second below the first, each filling width.
		expect(a && b && b.y > a.y + a.height - 2).toBeTruthy();
		expect(a && b && Math.abs(a.x - b.x) < 2).toBeTruthy();
		expect(a && a.width > 600).toBeTruthy();
	});
});

test.describe('kiosk split-screen crossfade', () => {
	// Landscape screen + a [solo, portrait, portrait] seed, so the pair is reached via a
	// solo→pair transition (unlike the portrait test, where the pair is the first slide).
	test.use({
		viewport: { width: 1000, height: 600 },
		serverOptions: { seedDir: splitImagesDir() }
	});

	test('completes a solo→pair crossfade so the pair becomes visible', async ({ kiosk, page }) => {
		await kiosk.goto();
		await kiosk.waitForImage();
		const panes = page.getByTestId('kiosk-slide-bottom').locator('> img');
		await expect.poll(() => panes.count(), { timeout: 15_000 }).toBe(2);
	});
});
