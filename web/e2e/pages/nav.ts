import type { Locator, Page } from '@playwright/test';

// Shared admin chrome (rail on desktop, bar on mobile). Logout exists in both;
// resolve to whichever is visible at the current viewport.
export class NavComponent {
	readonly logout: Locator;
	readonly bottom: Locator;
	readonly rail: Locator;

	constructor(private readonly page: Page) {
		this.logout = page
			.getByTestId('logout-rail')
			.or(page.getByTestId('logout-bar'))
			.filter({ visible: true });
		this.bottom = page.getByTestId('nav-bottom');
		this.rail = page.getByTestId('nav-rail');
	}

	async clickLogout(): Promise<void> {
		await this.logout.click();
	}

	// Distinct per nav; resolve the one visible at the current viewport.
	link(label: string): Locator {
		return this.page
			.getByTestId(`nav-rail-link-${label}`)
			.or(this.page.getByTestId(`nav-bottom-link-${label}`))
			.filter({ visible: true });
	}
}
