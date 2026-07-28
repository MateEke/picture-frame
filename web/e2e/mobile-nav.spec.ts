import { expect, test } from './fixtures';

// Runs only on the mobile project (desktop projects testIgnore this file).
test('mobile shows the bottom nav and hides the rail', async ({ page, nav }) => {
	await page.goto('/admin');
	await expect(nav.bottom).toBeVisible();
	await expect(nav.rail).toBeHidden();
});

test('the bottom nav navigates between pages', async ({ page, nav }) => {
	await page.goto('/admin');
	await nav.link('Network').click();
	await expect(page).toHaveURL(/\/admin\/network$/);
	await nav.link('Settings').click();
	await expect(page).toHaveURL(/\/admin\/settings$/);
});
