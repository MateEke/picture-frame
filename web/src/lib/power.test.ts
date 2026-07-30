import { describe, it, expect, vi, afterEach } from 'vitest';
import { toaster } from './toaster';
import { requestPower } from './power';

vi.mock('./toaster', () => ({
	toaster: { error: vi.fn() }
}));

const mockReboot = vi.fn();
const mockShutdown = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiSystemReboot: (...args: unknown[]) => mockReboot(...args),
	apiSystemShutdown: (...args: unknown[]) => mockShutdown(...args)
}));

describe('power', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('requestPower', () => {
		it('calls the reboot endpoint for a reboot', async () => {
			mockReboot.mockResolvedValue({ error: undefined });

			await expect(requestPower('reboot')).resolves.toBe(true);
			expect(mockReboot).toHaveBeenCalled();
			expect(mockShutdown).not.toHaveBeenCalled();
		});

		it('calls the shutdown endpoint for a shutdown', async () => {
			mockShutdown.mockResolvedValue({ error: undefined });

			await expect(requestPower('shutdown')).resolves.toBe(true);
			expect(mockShutdown).toHaveBeenCalled();
			expect(mockReboot).not.toHaveBeenCalled();
		});

		it('does not toast on success', async () => {
			mockReboot.mockResolvedValue({ error: undefined });

			await requestPower('reboot');

			expect(toaster.error).not.toHaveBeenCalled();
		});

		// A 503 is the normal answer on a host missing the polkit rule.
		it('reports failure and toasts when the server refuses', async () => {
			mockShutdown.mockResolvedValue({ error: { status: 503 } });

			await expect(requestPower('shutdown')).resolves.toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Shutdown failed',
				description: 'Server returned an error'
			});
		});

		it('reports failure and toasts when the request throws', async () => {
			mockReboot.mockRejectedValue(new Error('network down'));

			await expect(requestPower('reboot')).resolves.toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Reboot failed',
				description: 'network down'
			});
		});

		it('falls back to a generic description for a non-Error throw', async () => {
			mockReboot.mockRejectedValue('boom');

			await expect(requestPower('reboot')).resolves.toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Reboot failed',
				description: 'Unknown error'
			});
		});
	});
});
