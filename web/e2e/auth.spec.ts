import { expect, test } from './fixtures';
import { ADMIN_PASSWORD as PASSWORD, ADMIN_PASSWORD_HASH as PASSWORD_HASH } from './credentials';

test.describe('auth (password set)', () => {
	test.use({ serverOptions: { passwordHash: PASSWORD_HASH } });

	test('redirects to login when gated', async ({ page }) => {
		await page.goto('/admin');
		await expect(page).toHaveURL(/\/login\?next=%2Fadmin$/);
	});

	test('logs in and reaches the dashboard', async ({ page, login, dashboard }) => {
		await page.goto('/admin');
		await login.login(PASSWORD);
		await expect(dashboard.heading).toBeVisible();
	});

	test('sets a session cookie on login', async ({ page, context, login }) => {
		await page.goto('/login');
		await login.login(PASSWORD);
		await expect(page).toHaveURL(/\/admin$/);
		const cookies = await context.cookies();
		expect(cookies.some((c) => c.name === 'pf_session')).toBe(true);
	});

	test('rejects a wrong password', async ({ page, login }) => {
		await page.goto('/login');
		await login.login('wrong-password');
		await expect(login.error).toBeVisible();
		await expect(page).toHaveURL(/\/login/);
	});

	test('logs out and clears the session', async ({ page, context, login, nav }) => {
		await page.goto('/admin');
		await login.login(PASSWORD);
		await expect(page).toHaveURL(/\/admin$/);
		await nav.clickLogout();
		await expect(page).toHaveURL(/\/login$/);
		const cookies = await context.cookies();
		expect(cookies.some((c) => c.name === 'pf_session')).toBe(false);
	});

	test('returns to the requested page after login (next=)', async ({ page, login }) => {
		await page.goto('/admin/settings');
		await expect(page).toHaveURL(/\/login\?next=%2Fadmin%2Fsettings$/);
		await login.login(PASSWORD);
		await expect(page).toHaveURL(/\/admin\/settings$/);
	});
});

test.describe('auth (no password)', () => {
	test('admin is ungated: no login, no session cookie, no logout', async ({
		page,
		context,
		dashboard,
		nav
	}) => {
		await page.goto('/admin');
		await expect(dashboard.heading).toBeVisible();
		await expect(nav.logout).toHaveCount(0);
		const cookies = await context.cookies();
		expect(cookies.some((c) => c.name === 'pf_session')).toBe(false);
	});
});

test.describe('password lifecycle', () => {
	test('enabling protection adds logout, which re-gates the admin', async ({
		page,
		settings,
		login,
		nav
	}) => {
		await settings.goto();
		await expect(nav.logout).toHaveCount(0);

		await settings.openSection('security');
		await settings.securityNew.fill(PASSWORD);
		await settings.securityConfirm.fill(PASSWORD);
		await settings.securitySave.click();
		await expect(nav.logout).toBeVisible();

		await nav.clickLogout();
		await expect(page).toHaveURL(/\/login$/);

		await page.goto('/admin/settings');
		await expect(page).toHaveURL(/\/login\?next=/);
		await login.login(PASSWORD);
		await expect(page).toHaveURL(/\/admin\/settings$/);
	});

	test.describe('starting protected', () => {
		test.use({ serverOptions: { passwordHash: PASSWORD_HASH } });

		test('disabling protection revokes the session and removes logout', async ({
			page,
			context,
			settings,
			login,
			nav
		}) => {
			await page.goto('/admin/settings');
			await login.login(PASSWORD);
			await expect(page).toHaveURL(/\/admin\/settings$/);
			await expect(nav.logout).toBeVisible();

			await settings.openSection('security');
			await settings.securityCurrent.fill(PASSWORD);
			await settings.securityDisable.click();

			await expect(nav.logout).toHaveCount(0);
			const cookies = await context.cookies();
			expect(cookies.some((c) => c.name === 'pf_session')).toBe(false);
		});
	});
});
