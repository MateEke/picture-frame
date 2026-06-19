import { expect, test } from './fixtures';

test.describe('updates', () => {
	test.describe('when an update is available', () => {
		test.use({ serverOptions: { updateLatest: 'v9.9.9' } });

		test('surfaces a dashboard panel with release notes', async ({ dashboard }) => {
			await dashboard.goto();
			await expect(dashboard.updatePanel).toBeVisible();
			await expect(dashboard.updateNotes).toHaveAttribute('href', /v9\.9\.9/);

			// The About modal stays informational (version + a manual check), not a 2nd update prompt.
			await dashboard.openAbout();
			await expect(dashboard.aboutVersion).toBeVisible();
			await expect(dashboard.aboutCheck).toBeVisible();
			await expect(dashboard.aboutGithub).toHaveAttribute('href', /github\.com/);
			await expect(dashboard.aboutDocs).toHaveAttribute('href', /pictureframe\.ekemate\.hu/);
		});

		test('installs from the dashboard, then the panel clears', async ({ dashboard }) => {
			await dashboard.goto();
			await dashboard.updateNow.click();
			await expect(dashboard.updateConfirm).toBeVisible();
			await dashboard.updateConfirmGo.click();

			// The mock applies + bumps the version; the panel is no longer available.
			await expect(dashboard.updatePanel).toBeHidden();
		});
	});

	test.describe('when the release source is unreachable', () => {
		test.use({ serverOptions: { updateOffline: true } });

		test('shows no dashboard panel and offers a manual check in about', async ({ dashboard }) => {
			await dashboard.goto();
			await expect(dashboard.updatePanel).toBeHidden();

			await dashboard.openAbout();
			await expect(dashboard.aboutCheck).toBeVisible(); // up-to-date/offline → manual check, no notice
		});
	});

	test.describe('when an update fails and rolls back', () => {
		test.use({
			serverOptions: { updateLatest: 'v9.9.9', updateOutcome: 'rolled back from v9.9.9' }
		});

		test('keeps the update on offer and flags the failed attempt', async ({ dashboard }) => {
			await dashboard.goto();
			await dashboard.updateNow.click();
			await dashboard.updateConfirmGo.click();

			await expect(dashboard.updateRetryNote).toBeVisible();
			await expect(dashboard.updateNow).toBeVisible();
		});
	});

	test.describe('automatic update settings', () => {
		test('the schedule follows the switch and advanced config is always present', async ({
			settings
		}) => {
			await settings.goto();
			await settings.openSection('updates');

			// Auto-update is on by default, so the schedule shows.
			await expect(settings.updateHour).toBeVisible();
			await expect(settings.githubRepo).toBeVisible(); // advanced config lives here

			// Toggling off hides the schedule and dirties the form.
			await settings.autoUpdateSwitch.click();
			await expect(settings.updateHour).toBeHidden();
			await expect(settings.saveBar).toBeVisible();
		});
	});
});
