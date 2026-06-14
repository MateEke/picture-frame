import { expect, test } from './fixtures';

test.describe('wifi', () => {
	test.beforeEach(async ({ wifi }) => {
		await wifi.goto();
		await expect(wifi.status).toBeVisible();
	});

	test('shows the active connection', async ({ wifi }) => {
		await expect(wifi.status).toContainText('Home-WiFi');
		await expect(wifi.status).toContainText('192.168.1.42');
		await expect(wifi.status).toContainText('82%');
	});

	test('scans and lists networks', async ({ wifi }) => {
		await wifi.scan.click();
		await expect(wifi.networks).toHaveCount(6);
	});

	test.describe('after scanning', () => {
		test.beforeEach(async ({ wifi }) => {
			await wifi.scan.click();
			await expect(wifi.networks).toHaveCount(6);
		});

		test('connects to a saved network in one click', async ({ wifi }) => {
			await wifi.connectButton('OldRouter').click();
			await expect(wifi.banner).toContainText('Connected to OldRouter', { timeout: 8000 });
		});

		test('connects to an open network without a password', async ({ wifi }) => {
			await wifi.connectButton('Coffee Shop').click();
			await expect(wifi.connectDialog).toHaveCount(0);
			await expect(wifi.banner).toContainText('Connected to Coffee Shop', { timeout: 8000 });
		});

		test('prompts for a password on a secured network', async ({ wifi }) => {
			await wifi.connectButton('Home-WiFi-5G').click();
			await expect(wifi.connectDialog).toBeVisible();
			await wifi.dialogPassword.fill('hunter2');
			await wifi.dialogConnect.click();
			await expect(wifi.banner).toContainText('Connected to Home-WiFi-5G', { timeout: 8000 });
		});

		test('marks WPA3-only networks unsupported (no connect)', async ({ wifi }) => {
			await expect(wifi.connectButton('SecureCorp')).toHaveCount(0);
		});

		test('forgets a saved network', async ({ wifi }) => {
			await wifi.forgetButton('OldRouter').click();
			await expect(wifi.forgetDialog).toBeVisible();
			await wifi.forgetConfirm.click();
			await expect(wifi.forgetButton('OldRouter')).toHaveCount(0);
		});
	});

	test.describe('AP fallback', () => {
		test('shows the hotspot settings', async ({ wifi }) => {
			await expect(wifi.apSsid).toHaveValue('PictureFrame');
			await expect(wifi.apSwitch).toContainText('Enabled');
		});

		test('toggles the hotspot off and on', async ({ wifi }) => {
			await wifi.apSwitch.click();
			await expect(wifi.apSwitch).toContainText('Disabled');
			await wifi.apSwitch.click();
			await expect(wifi.apSwitch).toContainText('Enabled');
		});

		test('renames the hotspot and persists it', async ({ page, wifi }) => {
			await wifi.apSsid.fill('GarageFrame');
			await wifi.apSave.click();
			await page.reload();
			await expect(wifi.apSsid).toHaveValue('GarageFrame');
		});

		test('clears the hotspot password (open network)', async ({ page, wifi }) => {
			// Mock seeds a password, so Clear is offered.
			await expect(wifi.apPasswordClear).toBeVisible();
			await wifi.apPasswordClear.click();
			await wifi.apSave.click();
			await page.reload();
			await expect(wifi.apPasswordClear).toHaveCount(0);
		});
	});
});

test.describe('wifi unavailable', () => {
	test.use({ serverOptions: { wifiOff: true } });

	test('shows a notice when the backend is off', async ({ wifi }) => {
		await wifi.goto();
		await expect(wifi.unavailable).toBeVisible();
	});
});
