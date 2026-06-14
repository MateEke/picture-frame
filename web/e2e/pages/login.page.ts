import type { Locator, Page } from '@playwright/test';

export class LoginPage {
	readonly password: Locator;
	readonly submit: Locator;
	readonly error: Locator;

	constructor(private readonly page: Page) {
		this.password = page.getByTestId('login-password');
		this.submit = page.getByTestId('login-submit');
		this.error = page.getByTestId('login-error');
	}

	async login(password: string): Promise<void> {
		await this.password.fill(password);
		await this.submit.click();
	}
}
