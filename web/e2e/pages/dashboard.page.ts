import type { Locator, Page } from '@playwright/test';

export class DashboardPage {
	readonly heading: Locator;
	readonly tileWifi: Locator;
	readonly tileWeather: Locator;
	readonly tileLibrary: Locator;
	readonly sensorReadings: Locator;
	readonly screenStatus: Locator;
	readonly screenSwitch: Locator;
	readonly systemHostname: Locator;
	readonly systemUptime: Locator;
	readonly nowPlaying: Locator;
	readonly restart: Locator;
	readonly restartDialog: Locator;
	readonly restartCancel: Locator;
	readonly restartConfirm: Locator;
	readonly version: Locator;
	readonly updatePanel: Locator;
	readonly updateNow: Locator;
	readonly updateNotes: Locator;
	readonly updateConfirm: Locator;
	readonly updateConfirmGo: Locator;
	readonly updateRetryNote: Locator;
	readonly updateRolledBack: Locator;
	readonly aboutModal: Locator;
	readonly aboutVersion: Locator;
	readonly aboutPlatform: Locator;
	readonly aboutCheck: Locator;
	readonly aboutLicensesToggle: Locator;
	readonly aboutLicenses: Locator;
	readonly aboutClose: Locator;

	constructor(private readonly page: Page) {
		this.heading = page.getByTestId('dashboard-heading');
		this.tileWifi = page.getByTestId('tile-wifi');
		this.tileWeather = page.getByTestId('tile-weather');
		this.tileLibrary = page.getByTestId('tile-library');
		this.sensorReadings = page.getByTestId('sensor-reading');
		this.screenStatus = page.getByTestId('screen-status');
		this.screenSwitch = page.getByTestId('screen-switch');
		this.systemHostname = page.getByTestId('system-hostname');
		this.systemUptime = page.getByTestId('system-uptime');
		this.nowPlaying = page.getByTestId('now-playing-image');
		this.restart = page.getByTestId('dashboard-restart');
		this.restartDialog = page.getByTestId('restart-dialog');
		this.restartCancel = page.getByTestId('restart-cancel');
		this.restartConfirm = page.getByTestId('restart-confirm');
		this.version = page.getByTestId('dashboard-version');
		this.updatePanel = page.getByTestId('update-panel');
		this.updateNow = page.getByTestId('update-now');
		this.updateNotes = page.getByTestId('update-notes');
		this.updateConfirm = page.getByTestId('update-confirm');
		this.updateConfirmGo = page.getByTestId('update-confirm-go');
		this.updateRetryNote = page.getByTestId('update-retry-note');
		this.updateRolledBack = page.getByTestId('update-rolled-back');
		this.aboutModal = page.getByTestId('about-modal');
		this.aboutVersion = page.getByTestId('about-version');
		this.aboutPlatform = page.getByTestId('about-platform');
		this.aboutCheck = page.getByTestId('about-check');
		this.aboutLicensesToggle = page.getByTestId('about-licenses-toggle');
		this.aboutLicenses = page.getByTestId('about-licenses');
		this.aboutClose = page.getByTestId('about-close');
	}

	async goto(): Promise<void> {
		await this.page.goto('/admin');
	}

	async toggleScreen(): Promise<void> {
		await this.screenSwitch.click();
	}

	async openAbout(): Promise<void> {
		await this.version.click();
		await this.aboutModal.waitFor();
	}
}
