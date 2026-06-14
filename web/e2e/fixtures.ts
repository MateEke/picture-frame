import { test as base, expect } from '@playwright/test';
import { startServer, type PfServer, type ServerOptions } from './server';
import { KioskPage } from './pages/kiosk.page';
import { DashboardPage } from './pages/dashboard.page';
import { LoginPage } from './pages/login.page';
import { NavComponent } from './pages/nav';
import { SettingsPage } from './pages/settings.page';
import { WifiPage } from './pages/wifi.page';
import { PhotosPage } from './pages/photos.page';

type Fixtures = {
	/** Per-describe server tuning; set via `test.use({ serverOptions: {...} })`. */
	serverOptions: ServerOptions;
	/** A fresh Go server (dev mode, mocks) spawned for each test. */
	pf: PfServer;
	kiosk: KioskPage;
	dashboard: DashboardPage;
	login: LoginPage;
	nav: NavComponent;
	settings: SettingsPage;
	wifi: WifiPage;
	photos: PhotosPage;
};

export const test = base.extend<Fixtures>({
	serverOptions: [{}, { option: true }],

	pf: async ({ serverOptions }, use) => {
		const server = await startServer(serverOptions);
		await use(server);
		await server.stop();
	},

	// Point the browser at this test's own server.
	baseURL: async ({ pf }, use) => {
		await use(pf.baseURL);
	},

	kiosk: async ({ page }, use) => {
		await use(new KioskPage(page));
	},

	dashboard: async ({ page }, use) => {
		await use(new DashboardPage(page));
	},

	login: async ({ page }, use) => {
		await use(new LoginPage(page));
	},

	nav: async ({ page }, use) => {
		await use(new NavComponent(page));
	},

	settings: async ({ page }, use) => {
		await use(new SettingsPage(page));
	},

	wifi: async ({ page }, use) => {
		await use(new WifiPage(page));
	},

	photos: async ({ page }, use) => {
		await use(new PhotosPage(page));
	}
});

export { expect };
