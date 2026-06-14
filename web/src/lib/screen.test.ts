import { describe, it, expect, vi, afterEach } from 'vitest';
import { toaster } from './toaster';
import { loadScreen, setScreen } from './screen';

vi.mock('./toaster', () => ({
	toaster: { error: vi.fn() }
}));

const mockGetScreen = vi.fn();
const mockSetScreen = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiGetScreen: (...args: unknown[]) => mockGetScreen(...args),
	apiSetScreen: (...args: unknown[]) => mockSetScreen(...args)
}));

describe('screen', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('loadScreen', () => {
		it('returns the parsed payload on ok response', async () => {
			mockGetScreen.mockResolvedValue({ data: { state: 'on', auto: true }, error: undefined });

			const result = await loadScreen(fetch);
			expect(result).toEqual({ state: 'on', auto: true });
		});

		it('passes fetch to the SDK call', async () => {
			const customFetch = vi.fn();
			mockGetScreen.mockResolvedValue({ data: { state: 'off', auto: false }, error: undefined });

			await loadScreen(customFetch);

			expect(mockGetScreen).toHaveBeenCalledWith(expect.objectContaining({ fetch: customFetch }));
		});

		it('throws when the request fails', async () => {
			mockGetScreen.mockResolvedValue({ data: undefined, error: { status: 500 } });

			await expect(loadScreen(fetch)).rejects.toThrow('Failed to load screen state');
		});
	});

	describe('setScreen', () => {
		it('sends state:on when called with true', async () => {
			mockSetScreen.mockResolvedValue({ error: undefined });

			await setScreen(true);

			expect(mockSetScreen).toHaveBeenCalledWith(
				expect.objectContaining({ body: { state: 'on' } })
			);
		});

		it('sends state:off when called with false', async () => {
			mockSetScreen.mockResolvedValue({ error: undefined });

			await setScreen(false);

			expect(mockSetScreen).toHaveBeenCalledWith(
				expect.objectContaining({ body: { state: 'off' } })
			);
		});

		it('does not toast on success', async () => {
			mockSetScreen.mockResolvedValue({ error: undefined });

			await setScreen(true);

			expect(toaster.error).not.toHaveBeenCalled();
		});

		it('toasts on error', async () => {
			mockSetScreen.mockResolvedValue({ error: { status: 500 } });

			await setScreen(true);

			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Could not toggle screen',
				description: 'Server returned an error'
			});
		});
	});
});
