import { expect, test } from './fixtures';

test.describe('kiosk touch navigation', () => {
	// A long dwell keeps auto-advance out of the assertions.
	test.use({ serverOptions: { slideshowInterval: '60s' } });

	test.beforeEach(async ({ kiosk }) => {
		await kiosk.goto();
		await kiosk.waitForImage();
	});

	test('tapping the right half advances', async ({ kiosk }) => {
		const first = String(await kiosk.currentImageSrc());
		await kiosk.tapNext.click();
		expect(await kiosk.waitForImageChange(first)).not.toBe(first);
	});

	test('tapping the left half steps back', async ({ kiosk }) => {
		const first = String(await kiosk.currentImageSrc());
		await kiosk.tapNext.click();
		const second = await kiosk.waitForImageChange(first);
		await kiosk.tapPrev.click();
		expect(await kiosk.waitForImageChange(second)).toBe(first);
	});

	test('a tap over the overlay still registers', async ({ kiosk }) => {
		const first = String(await kiosk.currentImageSrc());
		await kiosk.tapAt(0.75, 0.95); // inside the overlay band, right half
		expect(await kiosk.waitForImageChange(first)).not.toBe(first);
	});

	test('a tap on a blanked screen wakes it without advancing', async ({ kiosk, page, pf }) => {
		const first = String(await kiosk.currentImageSrc());
		const off = await page.request.post(`${pf.baseURL}/api/screen`, { data: { state: 'off' } });
		expect(off.ok()).toBeTruthy();
		await kiosk.waitForScreenOff();

		await kiosk.tapNext.click();

		await expect
			.poll(async () => {
				const res = await page.request.get(`${pf.baseURL}/api/screen`);
				return (await res.json()).state;
			})
			.toBe('on');
		expect(await kiosk.currentImageSrc()).toBe(first);
	});
});
