import { expect, test } from './fixtures';

test.describe('smoke', () => {
	test('root redirects to the admin dashboard', async ({ page, dashboard }) => {
		await page.goto('/');
		await expect(page).toHaveURL(/\/admin$/);
		await expect(dashboard.heading).toBeVisible();
	});

	test('admin deep-link resolves through the SPA fallback', async ({ page }) => {
		await page.goto('/admin/settings');
		await expect(page).toHaveURL(/\/admin\/settings$/);
	});

	test('kiosk renders a slideshow image from SSE', async ({ kiosk }) => {
		await kiosk.goto();
		await kiosk.waitForImage();
	});
});
