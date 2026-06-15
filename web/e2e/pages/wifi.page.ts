import type { Locator, Page } from '@playwright/test';

export class WifiPage {
	readonly status: Locator;
	readonly scan: Locator;
	readonly networks: Locator;
	readonly banner: Locator;
	readonly connectDialog: Locator;
	readonly dialogPassword: Locator;
	readonly dialogConnect: Locator;
	readonly forgetDialog: Locator;
	readonly forgetConfirm: Locator;
	readonly joinHidden: Locator;
	readonly hiddenDialog: Locator;
	readonly hiddenSsid: Locator;
	readonly hiddenPassword: Locator;
	readonly hiddenConnect: Locator;
	readonly unavailable: Locator;
	readonly apCard: Locator;
	readonly apSsid: Locator;
	readonly apSwitch: Locator;
	readonly apSave: Locator;
	readonly apPasswordClear: Locator;

	constructor(private readonly page: Page) {
		this.status = page.getByTestId('wifi-status');
		this.scan = page.getByTestId('wifi-scan');
		this.networks = page.getByTestId('wifi-net');
		this.banner = page.getByTestId('wifi-banner');
		this.connectDialog = page.getByTestId('wifi-connect-dialog');
		this.dialogPassword = page.getByTestId('wifi-dialog-password');
		this.dialogConnect = page.getByTestId('wifi-dialog-connect');
		this.forgetDialog = page.getByTestId('wifi-forget-dialog');
		this.forgetConfirm = page.getByTestId('wifi-forget-confirm');
		this.joinHidden = page.getByTestId('wifi-join-hidden');
		this.hiddenDialog = page.getByTestId('wifi-hidden-dialog');
		this.hiddenSsid = page.getByTestId('wifi-hidden-ssid');
		this.hiddenPassword = page.getByTestId('wifi-hidden-password');
		this.hiddenConnect = page.getByTestId('wifi-hidden-connect');
		this.unavailable = page.getByTestId('wifi-unavailable');
		this.apCard = page.getByTestId('wifi-ap');
		this.apSsid = page.getByTestId('ap-ssid');
		this.apSwitch = page.getByTestId('ap-switch');
		this.apSave = page.getByTestId('ap-save');
		this.apPasswordClear = page.getByTestId('ap-password-clear');
	}

	connectButton(ssid: string): Locator {
		return this.page.getByTestId(`wifi-connect-${ssid}`);
	}

	forgetButton(ssid: string): Locator {
		return this.page.getByTestId(`wifi-forget-${ssid}`);
	}

	async goto(): Promise<void> {
		await this.page.goto('/admin/wifi');
	}
}
