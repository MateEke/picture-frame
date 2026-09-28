import { expect, test } from './fixtures';

test.describe('kiosk touch navigation', () => {
	// A long dwell keeps auto-advance out of the assertions.
	test.use({ serverOptions: { slideshowInterval: '60s' } });

	test.beforeEach(async ({ kiosk }) => {
		await kiosk.goto();
		await kiosk.waitForImage();
	});

	test('swiping left advances', async ({ kiosk }) => {
		const first = String(await kiosk.currentImageSrc());
		await kiosk.swipe('left');
		expect(await kiosk.waitForImageChange(first)).not.toBe(first);
		await expect(kiosk.menu).toHaveCount(0);
	});

	test('swiping right steps back', async ({ kiosk }) => {
		const first = String(await kiosk.currentImageSrc());
		await kiosk.swipe('left');
		const second = await kiosk.waitForImageChange(first);
		await kiosk.swipe('right');
		expect(await kiosk.waitForImageChange(second)).toBe(first);
	});

	test('a tap opens the menu, and the start button returns to the slideshow', async ({
		kiosk,
		page
	}) => {
		await kiosk.tapAt(0.75, 0.95); // over the overlay band still registers
		await expect(kiosk.menu).toBeVisible();
		await page.getByTestId('touch-start-slideshow').click();
		await expect(kiosk.slideshow).toBeVisible();
		await kiosk.waitForImage();
	});

	test('a tap on a blanked screen wakes it without opening the menu', async ({
		kiosk,
		page,
		pf
	}) => {
		const off = await page.request.post(`${pf.baseURL}/api/screen`, { data: { state: 'off' } });
		expect(off.ok()).toBeTruthy();
		await kiosk.waitForScreenOff();

		await kiosk.tapAt(0.5, 0.5);

		await expect
			.poll(async () => {
				const res = await page.request.get(`${pf.baseURL}/api/screen`);
				return (await res.json()).state;
			})
			.toBe('on');
		await expect(kiosk.menu).toHaveCount(0);
	});
});
