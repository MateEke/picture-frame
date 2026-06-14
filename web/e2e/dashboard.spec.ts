import { expect, test } from './fixtures';

test.describe('dashboard', () => {
	test.beforeEach(async ({ dashboard }) => {
		await dashboard.goto();
		await expect(dashboard.heading).toBeVisible();
	});

	test('weather tile shows live SSE data', async ({ dashboard }) => {
		await expect(dashboard.tileWeather).toContainText('18.5');
		await expect(dashboard.tileWeather).toContainText('60%');
	});

	test('wifi tile shows the mock connection', async ({ dashboard }) => {
		await expect(dashboard.tileWifi).toContainText('Home-WiFi');
	});

	test('sensor readings render from SSE', async ({ dashboard }) => {
		await expect(dashboard.sensorReadings.first()).toBeVisible();
	});

	test('screen toggle flips live state and restores', async ({ dashboard }) => {
		await expect(dashboard.screenStatus).toHaveText('Screen on');
		await dashboard.toggleScreen();
		await expect(dashboard.screenStatus).toHaveText('Screen off');
		await dashboard.toggleScreen();
		await expect(dashboard.screenStatus).toHaveText('Screen on');
	});

	test('system card loads device info', async ({ dashboard }) => {
		await expect(dashboard.systemHostname).not.toHaveText('—');
		await expect(dashboard.systemUptime).not.toHaveText('—');
	});

	test('tiles link to their pages', async ({ page, dashboard }) => {
		await dashboard.tileLibrary.click();
		await expect(page).toHaveURL(/\/admin\/images$/);
		await dashboard.goto();
		await dashboard.tileWifi.click();
		await expect(page).toHaveURL(/\/admin\/wifi$/);
	});

	test('now playing card shows the current image', async ({ dashboard }) => {
		await expect(dashboard.nowPlaying).toHaveAttribute('src', /^\/img\//);
	});

	test('restart asks for confirmation', async ({ dashboard }) => {
		await dashboard.restart.click();
		await expect(dashboard.restartDialog).toBeVisible();
		await dashboard.restartCancel.click();
		await expect(dashboard.restartDialog).toBeHidden();
	});

	test('confirming restart re-execs the server and the dashboard reconnects', async ({
		page,
		dashboard
	}) => {
		await dashboard.restart.click();
		// The dialog polls /healthz after the re-exec, then reloads the page; waiting
		// for that load event proves the server actually went down and came back.
		const reloaded = page.waitForEvent('load', { timeout: 20_000 });
		await dashboard.restartConfirm.click();
		await reloaded;
		await expect(dashboard.heading).toBeVisible();
		await expect(dashboard.tileWeather).toContainText('18.5'); // SSE reconnected
	});
});
