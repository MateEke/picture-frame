import { defineConfig, devices } from '@playwright/test';

const CI = !!process.env.CI;

// Admin specs also run at mobile viewport; the kiosk is fixed-landscape.
const adminSpecs = [
	'**/dashboard.spec.ts',
	'**/settings.spec.ts',
	'**/wifi.spec.ts',
	'**/photos.spec.ts',
	'**/auth.spec.ts',
	'**/mobile-nav.spec.ts'
];

// Asserts the mobile layout, so the desktop projects skip it.
const mobileSpecs = '**/mobile-nav.spec.ts';

export default defineConfig({
	testDir: './e2e',
	testMatch: '**/*.spec.ts',
	fullyParallel: true,
	forbidOnly: CI,
	retries: CI ? 2 : 0,
	workers: CI ? 2 : undefined,
	reporter: CI ? [['github'], ['html', { open: 'never' }]] : 'list',
	globalSetup: './e2e/global-setup.ts',
	use: {
		// baseURL is provided per-test by the `pf` fixture.
		trace: 'on-first-retry'
	},
	// One static preview backs all per-test Go servers (stateless SPA, relative
	// API URLs). Port 5173 = the Go dev-proxy target.
	webServer: {
		command: 'npm run build && npm run preview -- --port 5173 --strictPort',
		port: 5173,
		reuseExistingServer: !CI,
		timeout: 180_000
	},
	projects: [
		{
			// System Chrome → no `playwright install` locally (Fedora can't run bundled WebKit).
			name: 'chromium',
			testIgnore: mobileSpecs,
			use: {
				...devices['Desktop Chrome'],
				channel: 'chrome',
				viewport: { width: 1280, height: 720 }
			}
		},
		{
			name: 'firefox',
			testIgnore: mobileSpecs,
			use: { ...devices['Desktop Firefox'], viewport: { width: 1280, height: 720 } }
		},
		{
			name: 'webkit',
			testIgnore: mobileSpecs,
			use: { ...devices['Desktop Safari'], viewport: { width: 1280, height: 720 } }
		},
		{
			name: 'mobile-chromium',
			use: { ...devices['Pixel 7'] },
			testMatch: adminSpecs
		}
	]
});
