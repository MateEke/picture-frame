import { describe, it, expect, vi, afterEach } from 'vitest';
import { isOnDeviceKiosk, nextSlide, prevSlide, tapAction, wakeScreen } from './slideNav';

const mockNext = vi.fn();
const mockPrev = vi.fn();
const mockWake = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiSlideshowNext: (...args: unknown[]) => mockNext(...args),
	apiSlideshowPrev: (...args: unknown[]) => mockPrev(...args),
	apiScreenWake: (...args: unknown[]) => mockWake(...args)
}));

describe('slideNav', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('tapAction', () => {
		it('advances on the right zone', () => {
			expect(tapAction('right', { busy: false, screenOff: false })).toBe('next');
		});

		it('steps back on the left zone', () => {
			expect(tapAction('left', { busy: false, screenOff: false })).toBe('prev');
		});

		it('wakes instead of navigating when the screen is off', () => {
			expect(tapAction('left', { busy: false, screenOff: true })).toBe('wake');
			expect(tapAction('right', { busy: false, screenOff: true })).toBe('wake');
		});

		it('ignores taps during a crossfade', () => {
			expect(tapAction('right', { busy: true, screenOff: false })).toBeNull();
			expect(tapAction('left', { busy: true, screenOff: false })).toBeNull();
		});

		it('still wakes a blanked screen during a crossfade', () => {
			expect(tapAction('right', { busy: true, screenOff: true })).toBe('wake');
		});
	});

	describe('isOnDeviceKiosk', () => {
		it('accepts the loopback hosts the kiosk browser uses', () => {
			expect(isOnDeviceKiosk('localhost')).toBe(true);
			expect(isOnDeviceKiosk('127.0.0.1')).toBe(true);
			expect(isOnDeviceKiosk('[::1]')).toBe(true);
		});

		it('rejects a remote viewer', () => {
			expect(isOnDeviceKiosk('192.168.1.20')).toBe(false);
			expect(isOnDeviceKiosk('frame.local')).toBe(false);
			expect(isOnDeviceKiosk('')).toBe(false);
		});
	});

	describe('requests', () => {
		it('posts each action to its route', () => {
			mockNext.mockResolvedValue({ error: undefined });
			mockPrev.mockResolvedValue({ error: undefined });
			mockWake.mockResolvedValue({ error: undefined });

			nextSlide();
			prevSlide();
			wakeScreen();

			expect(mockNext).toHaveBeenCalledOnce();
			expect(mockPrev).toHaveBeenCalledOnce();
			expect(mockWake).toHaveBeenCalledOnce();
		});

		// An unhandled rejection has nowhere to surface on a kiosk screen.
		it.each([
			['nextSlide', nextSlide, mockNext],
			['prevSlide', prevSlide, mockPrev],
			['wakeScreen', wakeScreen, mockWake]
		])('%s attaches a rejection handler', (_name, call, mock) => {
			const request = Promise.reject(new Error('offline'));
			const attach = vi.spyOn(request, 'catch');
			mock.mockReturnValue(request);

			expect(() => call()).not.toThrow();
			expect(attach).toHaveBeenCalledOnce();
		});
	});
});
