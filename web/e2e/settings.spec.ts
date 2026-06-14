import { expect, test } from './fixtures';
import { ADMIN_PASSWORD, ADMIN_PASSWORD_HASH } from './credentials';

test.describe('settings', () => {
	test.beforeEach(async ({ settings }) => {
		await settings.goto();
	});

	test('edits and saves a setting (persists across reload)', async ({ page, settings }) => {
		await settings.labelOutside.fill('E2E Changed');
		await expect(settings.saveBar).toContainText('Unsaved changes');
		await expect(settings.save).toBeEnabled();

		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();

		await page.reload();
		await expect(settings.labelOutside).toHaveValue('E2E Changed');
	});

	test('blocks save on invalid input', async ({ settings }) => {
		await settings.openSection('weather');
		await settings.weatherLat.fill('999');
		await expect(settings.issues).toContainText('Fix 1 issue to save');
		await expect(settings.save).toBeDisabled();
	});

	test('reverts a section', async ({ settings }) => {
		await settings.openSection('weather');
		await settings.weatherLat.fill('10');
		await expect(settings.saveBar).toBeVisible();

		await settings.revertSection('weather');
		await expect(settings.weatherLat).toHaveValue('47.3567');
		await expect(settings.saveBar).toBeHidden();
	});

	test('discards all changes', async ({ settings }) => {
		await settings.labelOutside.fill('E2E Changed');
		await expect(settings.saveBar).toBeVisible();

		await settings.discard.click();
		await expect(settings.saveBar).toBeHidden();
		await expect(settings.labelOutside).toHaveValue('E2E Outside');
	});

	test.describe('sensor dialog', () => {
		test.beforeEach(async ({ settings }) => {
			await settings.openSection('sensors');
		});

		test('adds a mock sensor', async ({ settings }) => {
			await settings.addSensor('e2e_sensor', 'garage');
			await expect(settings.sensorRow('e2e_sensor')).toBeVisible();
			await expect(settings.saveBar).toBeVisible();
		});

		test('blocks add on a duplicate id', async ({ settings }) => {
			await settings.sensorAdd.click();
			await settings.dialogId.fill('mock_inside');
			await settings.dialogRole.fill('garage');
			await expect(settings.dialogSave).toBeDisabled();
		});

		test('edits a sensor role', async ({ settings }) => {
			await settings.sensorEdit('mock_outside').click();
			await settings.dialogRole.fill('balcony');
			await settings.dialogSave.click();
			await expect(settings.sensorRow('mock_outside')).toContainText('balcony');
		});

		test('deletes a sensor', async ({ settings }) => {
			await settings.sensorDelete('mock_inside').click();
			await expect(settings.sensorRow('mock_inside')).toHaveCount(0);
		});

		test('cancel closes without adding', async ({ settings }) => {
			await settings.sensorAdd.click();
			await settings.dialogId.fill('temp_sensor');
			await settings.dialogCancel.click();
			await expect(settings.sensorRow('temp_sensor')).toHaveCount(0);
		});
	});

	test('immich backend reveals share fields and requires a URL', async ({ settings }) => {
		await settings.openSection('library');
		await settings.libraryBackend.selectOption('immich');
		await expect(settings.libraryShareUrl).toBeVisible();
		await expect(settings.save).toBeDisabled();

		await settings.libraryShareUrl.fill('https://immich.example.com/share/abc');
		await expect(settings.save).toBeEnabled();
	});

	test('mqtt bridge needs a broker, then reveals HA fields that are required', async ({
		settings
	}) => {
		await settings.openSection('mqtt');
		await expect(settings.mqttBrokerHint).toBeVisible();

		await settings.mqttBroker.fill('tcp://localhost:1883');
		await expect(settings.mqttBrokerHint).toBeHidden();
		await settings.mqttBridgeSwitch.click();
		await expect(settings.mqttNodeId).toBeVisible();

		// Bridge fields come pre-filled with defaults; clearing one blocks save.
		await settings.mqttNodeId.fill('');
		await expect(settings.save).toBeDisabled();
		await settings.mqttNodeId.fill('frame');
		await expect(settings.save).toBeEnabled();
	});

	test('a restart-required change surfaces the restart prompt', async ({ settings }) => {
		await settings.openSection('weather');
		await settings.weatherLat.fill('48'); // lat isn't live-applied → needs a restart
		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();

		await expect(settings.restartNow).toBeVisible();
		await settings.restartNow.click();
		await expect(settings.restartDialog).toBeVisible();
		await settings.restartCancel.click();
		await expect(settings.restartDialog).toBeHidden();
	});

	// The successful set/disable lifecycle lives in auth.spec; here we cover the
	// SecurityCard's validation.
	test('security blocks on a password mismatch', async ({ settings }) => {
		await settings.openSection('security');
		await settings.securityNew.fill('one-password');
		await settings.securityConfirm.fill('another-password');
		await expect(settings.securityMismatch).toBeVisible();
		await expect(settings.securitySave).toBeDisabled();
	});
});

// Separate top-level group: a password-set server gates /admin/settings, so it
// can't use the main describe's ungated goto beforeEach.
test.describe('settings security (already protected)', () => {
	test.use({ serverOptions: { passwordHash: ADMIN_PASSWORD_HASH } });

	test.beforeEach(async ({ page, login, settings }) => {
		await page.goto('/admin/settings');
		await login.login(ADMIN_PASSWORD);
		await settings.openSection('security');
	});

	test('change requires the current password', async ({ settings }) => {
		await settings.securityNew.fill('next-password');
		await settings.securityConfirm.fill('next-password');
		await expect(settings.securitySave).toBeDisabled();
		await settings.securityCurrent.fill(ADMIN_PASSWORD);
		await expect(settings.securitySave).toBeEnabled();
	});

	test('change rejects a wrong current password', async ({ settings }) => {
		await settings.securityCurrent.fill('wrong-password');
		await settings.securityNew.fill('next-password');
		await settings.securityConfirm.fill('next-password');
		await settings.securitySave.click();
		await expect(settings.securitySaveError).toBeVisible();
	});

	test('disable requires the current password', async ({ settings }) => {
		await expect(settings.securityDisable).toBeDisabled();
		await settings.securityCurrent.fill(ADMIN_PASSWORD);
		await expect(settings.securityDisable).toBeEnabled();
	});

	test('disable rejects a wrong current password', async ({ settings }) => {
		await settings.securityCurrent.fill('wrong-password');
		await settings.securityDisable.click();
		await expect(settings.securityDisableError).toBeVisible();
	});
});
