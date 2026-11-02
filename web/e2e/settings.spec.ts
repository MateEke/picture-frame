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

	test('toggles split-screen pairing and persists across reload', async ({ page, settings }) => {
		const input = settings.splitScreenSwitch.locator('input[type="checkbox"]');
		const before = await input.isChecked();
		await settings.splitScreenSwitch.click();
		await expect(input).toBeChecked({ checked: !before });
		await expect(settings.save).toBeEnabled();

		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();
		// It applies live, so saving must not prompt for a restart.
		await expect(settings.restartDialog).toBeHidden();

		await page.reload();
		await expect(settings.splitScreenSwitch.locator('input[type="checkbox"]')).toBeChecked({
			checked: !before
		});
	});

	test('toggles hide clock and date, applies live (no restart)', async ({ page, settings }) => {
		const input = settings.hideClockDateSwitch.locator('input[type="checkbox"]');
		const before = await input.isChecked();
		await settings.hideClockDateSwitch.click();
		await expect(input).toBeChecked({ checked: !before });
		await expect(settings.save).toBeEnabled();

		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();
		await expect(settings.restartDialog).toBeHidden();

		await page.reload();
		await expect(settings.hideClockDateSwitch.locator('input[type="checkbox"]')).toBeChecked({
			checked: !before
		});
	});

	test('sets a screen schedule live and persists across reload', async ({ page, settings }) => {
		await expect(settings.scheduleSummary).toHaveText(
			'No schedule. Set a time to turn the screen off or on every day.'
		);
		await settings.scheduleOff.fill('23:00');
		await expect(settings.scheduleSummary).toHaveText(
			'Turns off every day at 11:00 PM and stays off until you turn it back on.'
		);
		await settings.scheduleOn.fill('07:00');
		await expect(settings.scheduleSummary).toHaveText('Off every night from 11:00 PM to 7:00 AM.');

		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();
		await expect(settings.restartDialog).toBeHidden();

		await page.reload();
		await expect(settings.scheduleOff).toHaveValue('23:00');
		await expect(settings.scheduleOn).toHaveValue('07:00');
	});

	test('clears a schedule time', async ({ settings }) => {
		await settings.scheduleOff.fill('23:00');
		await settings.scheduleOffClear.click();
		await expect(settings.scheduleOff).toHaveValue('');
		await expect(settings.scheduleOffClear).toBeHidden();
		await expect(settings.scheduleSummary).toHaveText(
			'No schedule. Set a time to turn the screen off or on every day.'
		);
	});

	test('blocks saving equal schedule times', async ({ settings }) => {
		await settings.scheduleOff.fill('07:00');
		await settings.scheduleOn.fill('07:00');
		await expect(settings.scheduleSummary).toBeHidden();
		await expect(settings.issues).toBeVisible();
		await expect(settings.save).toBeDisabled();
	});

	test('selects a time zone and persists across reload', async ({ page, settings }) => {
		await settings.timezone.click();
		await settings.timezone.fill('Asia/Tokyo');
		await page.getByRole('option', { name: /Asia\/Tokyo/ }).click();
		await expect(settings.save).toBeEnabled();

		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();
		// Live field: no restart prompt.
		await expect(settings.restartDialog).toBeHidden();

		await page.reload();
		await expect(settings.timezone).toHaveValue(/Asia\/Tokyo/);
	});

	test('changes rotation, applies live and persists', async ({ page, settings }) => {
		await settings.openSection('display');
		await settings.rotation.selectOption('90');
		await expect(settings.save).toBeEnabled();

		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();
		// Live field: no restart prompt.
		await expect(settings.restartDialog).toBeHidden();

		await page.reload();
		await settings.openSection('display');
		await expect(settings.rotation).toHaveValue('90');
	});

	test('links to the manual', async ({ settings }) => {
		await expect(settings.docs).toHaveAttribute('href', /pictureframe\.ekemate\.hu\/manual/);
	});

	test('sets a custom slide interval and keeps it on the slider', async ({ page, settings }) => {
		await settings.enterInterval('15', 'sec');
		await expect(settings.intervalValue).toHaveText('15 sec');
		await settings.intervalAmount.press('Enter');
		await expect(settings.intervalAmount).toBeHidden();
		await expect(settings.intervalValue).toBeFocused();

		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();
		await page.reload();
		await expect(settings.intervalValue).toHaveText('15 sec');

		await settings.intervalThumb.press('ArrowRight');
		await expect(settings.intervalValue).toHaveText('30 sec');
		await settings.intervalThumb.press('ArrowLeft');
		await expect(settings.intervalValue).toHaveText('15 sec');
	});

	test('the interval editor opens in the largest whole unit and shows the range', async ({
		settings
	}) => {
		await settings.enterInterval('90', 'min');
		await settings.intervalAmount.press('Enter');
		await expect(settings.intervalValue).toHaveText('1 hr 30 min');

		await settings.intervalValue.click();
		await expect(settings.intervalAmount).toHaveValue('90');
		await expect(settings.intervalUnit).toHaveValue('min');
		await expect(settings.intervalHint).toHaveText('5 sec to 1 day');
		await expect(settings.intervalAmount).toHaveAccessibleDescription('5 sec to 1 day');
	});

	test('the interval readout is named, tappable and its editor fits a narrow screen', async ({
		page,
		settings
	}) => {
		await expect(settings.intervalValue).toHaveAccessibleName(/^Advance photo every: .+, edit$/);
		const box = await settings.intervalValue.boundingBox();
		expect(box?.height ?? 0).toBeGreaterThanOrEqual(24);

		await settings.intervalValue.click();
		await expect(settings.intervalUnit).toBeInViewport();
		const overflow = await page.evaluate(
			() => document.documentElement.scrollWidth - document.documentElement.clientWidth
		);
		expect(overflow).toBeLessThanOrEqual(0);
	});

	test('Esc restores the interval from before editing', async ({ settings }) => {
		await settings.enterInterval('65', 'min');
		await expect(settings.intervalValue).toHaveText('1 hr 5 min');
		await settings.intervalAmount.press('Escape');
		await expect(settings.intervalAmount).toBeHidden();
		await expect(settings.intervalValue).toHaveText('2 sec');
		await expect(settings.saveBar).toBeHidden();
	});

	test('finishing on the original interval keeps it, even below the minimum', async ({
		settings
	}) => {
		await settings.intervalValue.click();
		await settings.intervalAmount.press('Enter');
		await expect(settings.intervalAmount).toBeHidden();

		await settings.enterInterval('20', 'sec');
		await settings.intervalAmount.fill('2');
		await settings.intervalAmount.press('Enter');
		await expect(settings.intervalValue).toHaveText('2 sec');
		await expect(settings.saveBar).toBeHidden();
	});

	test('discarding while the interval editor is open closes it', async ({ settings }) => {
		await settings.enterInterval('5', 'min');
		await expect(settings.intervalValue).toHaveText('5 min');
		await settings.discard.click();
		await expect(settings.intervalAmount).toBeHidden();
		await expect(settings.intervalValue).toHaveText('2 sec');
		await expect(settings.saveBar).toBeHidden();
	});

	test('tapping elsewhere closes the interval editor and keeps the value', async ({ settings }) => {
		await settings.enterInterval('15', 'sec');
		await settings.labelOutside.click();
		await expect(settings.intervalAmount).toBeHidden();
		await expect(settings.intervalValue).toHaveText('15 sec');
	});

	test('an invalid interval keeps the editor open and drops the half-typed value', async ({
		settings
	}) => {
		await settings.enterInterval('2', 'hr');
		await expect(settings.intervalValue).toHaveText('2 hr');
		await settings.intervalAmount.fill('25');
		await settings.intervalAmount.press('Enter');
		await expect(settings.intervalError).toHaveText('Maximum is 1 day.');
		await expect(settings.intervalHint).toBeHidden();
		await expect(settings.intervalAmount).toHaveAttribute('aria-invalid', 'true');
		await expect(settings.intervalValue).toHaveText('2 sec');
		await expect(settings.saveBar).toBeHidden();

		await settings.labelOutside.click();
		await expect(settings.intervalAmount).toBeVisible();

		await settings.intervalAmount.fill('20');
		await expect(settings.intervalError).toBeHidden();
		await expect(settings.intervalValue).toHaveText('20 hr');
	});

	test('choosing a preset keeps the saved custom interval on the slider', async ({ settings }) => {
		await expect(settings.intervalValue).toHaveText('2 sec');
		await settings.enterInterval('1', 'min');
		await settings.intervalAmount.press('Enter');
		await expect(settings.intervalValue).toHaveText('1 min');

		await settings.intervalThumb.press('Home');
		await expect(settings.intervalValue).toHaveText('2 sec');
	});

	test('discarding drops a typed custom interval from the slider', async ({ settings }) => {
		await settings.enterInterval('15', 'sec');
		await settings.intervalAmount.press('Enter');
		await expect(settings.intervalValue).toHaveText('15 sec');

		await settings.discard.click();
		await expect(settings.intervalValue).toHaveText('2 sec');
		await settings.intervalThumb.press('ArrowRight');
		await expect(settings.intervalValue).toHaveText('30 sec');
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

	// Kit 3 re-runs loads on a same-page link click; that must not wipe a draft.
	test('keeps unsaved edits when the current nav link is clicked', async ({
		page,
		settings,
		nav
	}) => {
		await settings.labelOutside.fill('E2E Draft');
		await expect(settings.saveBar).toContainText('Unsaved changes');

		// Assert only after the refreshed config lands, or the check races it.
		const refreshed = page.waitForResponse((r) => new URL(r.url()).pathname === '/api/config');
		await nav.link('Settings').click();
		await refreshed;
		await expect(settings.labelOutside).toHaveValue('E2E Draft');
		await expect(settings.saveBar).toContainText('Unsaved changes');
	});

	// A replaced secret round-trips identically (omitted, *_set true), so the
	// save itself must reseed or the bar sticks.
	test('clears the save bar after replacing an already-set secret', async ({ settings }) => {
		await settings.openSection('weather');
		for (const key of ['e2e-key-one', 'e2e-key-two']) {
			await settings.weatherApiKey.fill(key);
			await settings.save.click();
			await expect(settings.saveBar).toBeHidden();
			await settings.restartCancel.click();
			await expect(settings.weatherApiKey).toHaveValue('');
		}
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

		test('Esc in a duration editor keeps the sensor dialog open', async ({ settings }) => {
			await settings.sensorAdd.click();
			await settings.dialogId.fill('e2e_esc');
			await settings.mockPollValue.click();
			await settings.mockPollAmount.fill('45');
			await settings.mockPollAmount.press('Escape');
			await expect(settings.mockPollAmount).toBeHidden();
			await expect(settings.dialogId).toHaveValue('e2e_esc');
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

	test('mqtt stale after can be set to Never', async ({ settings }) => {
		await settings.openSection('mqtt');
		await settings.mqttBroker.fill('tcp://localhost:1883');
		await settings.mqttBridgeSwitch.click();

		await settings.mqttStaleThumb.press('Home');
		await expect(settings.mqttStaleValue).toHaveText('Never');

		await settings.mqttStaleThumb.press('End');
		await settings.mqttStaleValue.click();
		await settings.mqttStaleAmount.fill('0');
		await settings.mqttStaleAmount.press('Enter');
		await expect(settings.mqttStaleValue).toHaveText('Never');
	});

	test('a restart-required change surfaces the restart prompt', async ({ settings }) => {
		await settings.openSection('weather');
		await settings.weatherLat.fill('48'); // lat isn't live-applied → needs a restart
		await settings.save.click();
		await expect(settings.saveBar).toBeHidden();

		await expect(settings.restartDialog).toBeVisible();
		await settings.restartCancel.click();
		await expect(settings.restartDialog).toBeHidden();

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

// Own server: the dev mock rotator reports no wlr-randr.
test.describe('rotation unsupported', () => {
	test.use({ serverOptions: { rotationUnsupported: true } });

	test('rotation select is disabled with the install hint', async ({ settings }) => {
		await settings.goto();
		await settings.openSection('display');
		await expect(settings.rotation).toBeDisabled();
		await expect(settings.rotationHint).toBeVisible();
	});
});
