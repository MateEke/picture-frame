import type { Locator, Page } from '@playwright/test';

export class SettingsPage {
	readonly saveBar: Locator;
	readonly save: Locator;
	readonly discard: Locator;
	readonly issues: Locator;
	readonly labelOutside: Locator;
	readonly weatherLat: Locator;
	readonly sensorAdd: Locator;
	readonly dialogId: Locator;
	readonly dialogRole: Locator;
	readonly dialogSave: Locator;
	readonly dialogCancel: Locator;
	readonly libraryBackend: Locator;
	readonly libraryShareUrl: Locator;
	readonly mqttBroker: Locator;
	readonly mqttBrokerHint: Locator;
	readonly mqttBridgeSwitch: Locator;
	readonly mqttNodeId: Locator;
	readonly securityNew: Locator;
	readonly securityConfirm: Locator;
	readonly securityCurrent: Locator;
	readonly securitySave: Locator;
	readonly securityDisable: Locator;
	readonly securityMismatch: Locator;
	readonly securitySaveError: Locator;
	readonly securityDisableError: Locator;
	readonly restartNow: Locator;
	readonly restartDialog: Locator;
	readonly restartCancel: Locator;
	readonly autoUpdateSwitch: Locator;
	readonly updateHour: Locator;
	readonly githubRepo: Locator;

	constructor(private readonly page: Page) {
		this.saveBar = page.getByTestId('settings-save-bar');
		this.save = page.getByTestId('settings-save');
		this.discard = page.getByTestId('settings-discard');
		this.issues = page.getByTestId('settings-issues');
		this.labelOutside = page.getByTestId('setting-label-outside');
		this.weatherLat = page.getByTestId('setting-weather-lat');
		this.sensorAdd = page.getByTestId('sensor-add');
		this.dialogId = page.getByTestId('sensor-dialog-id');
		this.dialogRole = page.getByTestId('sensor-dialog-role');
		this.dialogSave = page.getByTestId('sensor-dialog-save');
		this.dialogCancel = page.getByTestId('sensor-dialog-cancel');
		this.libraryBackend = page.getByTestId('library-backend');
		this.libraryShareUrl = page.getByTestId('library-share-url');
		this.mqttBroker = page.getByTestId('mqtt-broker');
		this.mqttBrokerHint = page.getByTestId('mqtt-broker-hint');
		this.mqttBridgeSwitch = page.getByTestId('mqtt-bridge-switch');
		this.mqttNodeId = page.getByTestId('mqtt-node-id');
		this.securityNew = page.getByTestId('security-new');
		this.securityConfirm = page.getByTestId('security-confirm');
		this.securityCurrent = page.getByTestId('security-current');
		this.securitySave = page.getByTestId('security-save');
		this.securityDisable = page.getByTestId('security-disable');
		this.securityMismatch = page.getByTestId('security-mismatch');
		this.securitySaveError = page.getByTestId('security-save-error');
		this.securityDisableError = page.getByTestId('security-disable-error');
		this.restartNow = page.getByTestId('settings-restart-now');
		this.restartDialog = page.getByTestId('restart-dialog');
		this.restartCancel = page.getByTestId('restart-cancel');
		this.autoUpdateSwitch = page.getByTestId('auto-update-switch');
		this.updateHour = page.getByTestId('update-hour');
		this.githubRepo = page.getByTestId('github-repo');
	}

	sensorRow(id: string): Locator {
		return this.page.getByTestId(`sensor-row-${id}`);
	}

	sensorEdit(id: string): Locator {
		return this.page.getByTestId(`sensor-edit-${id}`);
	}

	sensorDelete(id: string): Locator {
		return this.page.getByTestId(`sensor-delete-${id}`);
	}

	async goto(): Promise<void> {
		await this.page.goto('/admin/settings');
		await this.labelOutside.waitFor();
	}

	// The trigger toggles, so only click when the section is collapsed.
	async openSection(value: string): Promise<void> {
		const trigger = this.page.getByTestId(`settings-section-${value}`);
		if ((await trigger.getAttribute('aria-expanded')) !== 'true') {
			await trigger.click();
		}
	}

	async revertSection(value: string): Promise<void> {
		await this.page.getByTestId(`settings-revert-${value}`).click();
	}

	async addSensor(id: string, role: string): Promise<void> {
		await this.sensorAdd.click();
		await this.dialogId.fill(id);
		await this.dialogRole.fill(role);
		await this.dialogSave.click();
	}
}
