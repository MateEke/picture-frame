import { expect, test } from './fixtures';

// The on-device touch UI on a Touch Display 2-shaped viewport (720x1280 portrait).
test.describe('touch UI', () => {
	test.use({
		viewport: { width: 720, height: 1280 },
		serverOptions: { slideshowInterval: '60s' }
	});

	test.beforeEach(async ({ kiosk }) => {
		await kiosk.goto();
		await kiosk.waitForImage();
		await kiosk.openMenu();
	});

	test('home shows the clock and photo count', async ({ page }) => {
		await expect(page.getByTestId('touch-home')).toBeVisible();
		await expect(page.getByTestId('touch-home-counts')).toContainText('3');
	});

	test('gallery hides a photo from the slideshow', async ({ kiosk, page, pf }) => {
		await kiosk.tab('gallery').click();
		const items = page.getByTestId('touch-gallery-item');
		await expect(items).toHaveCount(3);

		await page.getByTestId('touch-gallery-select').click();
		await items.first().click();
		await expect(page.getByTestId('touch-gallery-selected')).toHaveText('1 valittu');
		await page.getByTestId('touch-gallery-hide').click();

		await expect
			.poll(async () => {
				const res = await page.request.get(`${pf.baseURL}/api/images`);
				const list: { included: boolean }[] = await res.json();
				return list.filter((i) => !i.included).length;
			})
			.toBe(1);

		await page.getByTestId('touch-gallery-filter-hidden').click();
		await expect(items).toHaveCount(1);
		await expect(items.first()).toHaveAttribute('data-included', 'false');
	});

	test('gallery viewer pages through photos', async ({ kiosk, page }) => {
		await kiosk.tab('gallery').click();
		await page.getByTestId('touch-gallery-item').first().click();
		const img = page.getByTestId('touch-viewer-img');
		const first = await img.getAttribute('src');
		await page.getByTestId('touch-viewer-next').click();
		await expect(img).not.toHaveAttribute('src', String(first));
		await page.getByTestId('touch-viewer-close').click();
		await expect(page.getByTestId('touch-viewer')).toHaveCount(0);
	});

	test('upload view shows the address to send photos to', async ({ kiosk, page }) => {
		await kiosk.tab('upload').click();
		await expect(page.getByTestId('touch-qr')).toBeVisible();
		await expect(page.getByTestId('touch-upload-url')).toContainText('/admin/images');
	});

	test('files view lists, previews and deletes an uploaded file', async ({ kiosk, page, pf }) => {
		const res = await page.request.post(`${pf.baseURL}/api/files`, {
			multipart: {
				file: { name: 'Muistiinpanot.txt', mimeType: 'text/plain', buffer: Buffer.from('Hei!') }
			}
		});
		expect(res.status()).toBe(201);

		await kiosk.tab('files').click();
		const item = page.getByTestId('touch-file-item');
		await expect(item).toHaveCount(1);
		await expect(item).toContainText('Muistiinpanot.txt');

		await item.click();
		await expect(page.getByTestId('touch-file-preview')).toContainText('Hei!');
		await page.getByTestId('touch-file-delete').click();
		await page.getByTestId('touch-confirm-ok').click();
		await expect(item).toHaveCount(0);
	});

	test('settings save themselves', async ({ kiosk, page, pf }) => {
		await kiosk.tab('settings').click();
		const value = page.getByTestId('touch-set-interval-value');
		await expect(value).toHaveText('1 min');
		await page.getByTestId('touch-set-interval-up').click();
		await expect(value).toHaveText('2 min');
		await expect(page.getByTestId('touch-notice')).toHaveText('Tallennettu');

		const res = await page.request.get(`${pf.baseURL}/api/touch/settings`);
		expect((await res.json()).interval).toBe('2m0s');
	});

	test('the slideshow takes over after the idle delay', async ({ kiosk, page, pf }) => {
		const current = await (await page.request.get(`${pf.baseURL}/api/touch/settings`)).json();
		const put = await page.request.put(`${pf.baseURL}/api/touch/settings`, {
			data: {
				sleep: { ...current.sleep, idle_after: '2s' },
				interval: current.interval,
				randomize: current.randomize,
				split_screen: current.split_screen,
				brightness: current.brightness,
				rotation: current.rotation
			}
		});
		expect(put.status()).toBe(204);
		await expect(kiosk.menu).toBeVisible();
		await expect(kiosk.slideshow).toBeVisible({ timeout: 10_000 });
	});
});
