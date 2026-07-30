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
		await expect(dashboard.tileNetwork).toContainText('Home-WiFi');
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

	test('system card shows Pi host metrics', async ({ dashboard }) => {
		// Memory and system uptime read from /proc, present on any Linux host.
		await expect(dashboard.systemMemory).not.toHaveText('—');
		await expect(dashboard.systemUptimeSystem).not.toHaveText('—');
		// CPU temp and power depend on Pi-only sources, so only assert they render.
		await expect(dashboard.systemCpuTemp).toBeVisible();
		await expect(dashboard.systemPower).toBeVisible();
	});

	test('tiles link to their pages', async ({ page, dashboard }) => {
		await dashboard.tileLibrary.click();
		await expect(page).toHaveURL(/\/admin\/images$/);
		await dashboard.goto();
		await dashboard.tileNetwork.click();
		await expect(page).toHaveURL(/\/admin\/network$/);
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

test.describe('host power controls', () => {
	test.beforeEach(async ({ dashboard }) => {
		await dashboard.goto();
		await expect(dashboard.heading).toBeVisible();
	});

	test('reboot asks for confirmation and can be cancelled', async ({ dashboard }) => {
		await dashboard.reboot.click();
		await expect(dashboard.rebootDialog).toBeVisible();
		await expect(dashboard.rebootDialog).toContainText('whole device restarts');
		await dashboard.rebootDialog.getByRole('button', { name: 'Cancel' }).click();
		await expect(dashboard.rebootDialog).toBeHidden();
	});

	// Shutdown is unrecoverable, so the wording has to say so plainly.
	test('shutdown warns that the frame will not come back on its own', async ({ dashboard }) => {
		await dashboard.shutdown.click();
		await expect(dashboard.shutdownDialog).toBeVisible();
		await expect(dashboard.shutdownDialog).toContainText('physically power it on again');
		await dashboard.shutdownDialog.getByRole('button', { name: 'Cancel' }).click();
		await expect(dashboard.shutdownDialog).toBeHidden();
	});

	// The dev mock never touches the host; this only proves the request lands.
	test('confirming shutdown reports progress', async ({ dashboard }) => {
		await dashboard.shutdown.click();
		await dashboard.shutdownConfirm.click();
		await expect(dashboard.shutdownDialog).toContainText('Powering off');
	});

	// A real reboot answers /healthz for seconds while systemd stops units, so the
	// dialog must not reload on the first success. The dev mock never goes down,
	// which is the worst case: it must keep waiting rather than reload.
	test('waiting for a reboot does not reload while the server still answers', async ({
		page,
		dashboard
	}) => {
		let reloads = 0;
		page.on('load', () => reloads++);

		await dashboard.reboot.click();
		await dashboard.rebootConfirm.click();
		await expect(dashboard.rebootDialog).toContainText('Waiting for the frame');

		await page.waitForTimeout(5000); // two poll cycles
		await expect(dashboard.rebootDialog).toContainText('Waiting for the frame');
		expect(reloads).toBe(0);
	});
});

// Own server: the dev mock reports no polkit rule.
test.describe('host power denied', () => {
	test.use({ serverOptions: { powerDenied: true } });

	test('reboot and shutdown are hidden, restart still works', async ({ dashboard }) => {
		await dashboard.goto();
		await expect(dashboard.heading).toBeVisible();
		await expect(dashboard.reboot).toBeHidden();
		await expect(dashboard.shutdown).toBeHidden();
		await expect(dashboard.restart).toBeVisible();
	});
});
