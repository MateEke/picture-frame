import { apiScreenWake, apiSlideshowNext, apiSlideshowPrev } from '$lib/api/sdk.gen';

export type TapZone = 'left' | 'right';
export type TapAction = 'wake' | 'next' | 'prev' | null;

export function tapAction(zone: TapZone, opts: { busy: boolean; screenOff: boolean }): TapAction {
	if (opts.screenOff) return 'wake';
	if (opts.busy) return null;
	return zone === 'left' ? 'prev' : 'next';
}

/** The kiosk browser loads http://localhost/kiosk. A remote viewer must not drive the frame. */
export function isOnDeviceKiosk(hostname: string): boolean {
	return hostname === 'localhost' || hostname === '127.0.0.1' || hostname === '[::1]';
}

function fire(request: Promise<unknown>): void {
	request.catch(() => undefined);
}

export function nextSlide(): void {
	fire(apiSlideshowNext());
}

export function prevSlide(): void {
	fire(apiSlideshowPrev());
}

export function wakeScreen(): void {
	fire(apiScreenWake());
}
