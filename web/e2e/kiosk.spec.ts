import { expect, test } from './fixtures';

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
