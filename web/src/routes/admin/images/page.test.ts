import { describe, it, expect, vi, beforeEach } from 'vitest';
import { load } from './+page';

const mockLoadImages = vi.fn();
const mockLoadLibrary = vi.fn();
const mockLoadConfig = vi.fn();

vi.mock('$lib/images', () => ({ loadImages: (...a: unknown[]) => mockLoadImages(...a) }));
vi.mock('$lib/library', () => ({ loadLibrary: (...a: unknown[]) => mockLoadLibrary(...a) }));
vi.mock('$lib/config', () => ({ loadConfig: (...a: unknown[]) => mockLoadConfig(...a) }));

// The loader only reads `fetch`; cast the minimal event once.
function event() {
	return { fetch: vi.fn() } as unknown as Parameters<typeof load>[0];
}

describe('images page load', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockLoadImages.mockResolvedValue([{ name: 'a.jpg' }]);
		mockLoadLibrary.mockResolvedValue({ backend: 'fs' });
		mockLoadConfig.mockResolvedValue({
			library: { immich: { share_url: null } },
			slideshow: { randomize: false }
		});
	});

	it('fetches config for fs backend, shareUrl is null, shuffleOn reflects randomize', async () => {
		const result = await load(event());
		expect(mockLoadConfig).toHaveBeenCalledOnce();
		expect(result).toEqual({
			images: [{ name: 'a.jpg' }],
			imagesError: undefined,
			library: { backend: 'fs' },
			libraryError: undefined,
			shareUrl: null,
			shuffleOn: false
		});
	});

	it('exposes shuffleOn true for fs when randomize is true', async () => {
		mockLoadConfig.mockResolvedValue({
			library: { immich: { share_url: null } },
			slideshow: { randomize: true }
		});

		const result = await load(event());
		expect(mockLoadConfig).toHaveBeenCalledOnce();
		expect(result).toMatchObject({ shareUrl: null, shuffleOn: true });
	});

	it('fetches config and exposes shareUrl and shuffleOn for the immich backend', async () => {
		mockLoadLibrary.mockResolvedValue({ backend: 'immich' });
		mockLoadConfig.mockResolvedValue({
			library: { immich: { share_url: 'https://share' } },
			slideshow: { randomize: false }
		});

		const result = await load(event());
		expect(mockLoadConfig).toHaveBeenCalledOnce();
		expect(result).toMatchObject({ shareUrl: 'https://share', shuffleOn: false });
	});

	it('falls back to null shareUrl and false shuffleOn when config is null', async () => {
		mockLoadLibrary.mockResolvedValue({ backend: 'immich' });
		mockLoadConfig.mockResolvedValue(null);

		expect(await load(event())).toMatchObject({ shareUrl: null, shuffleOn: false });
	});

	it('reports an images failure while still returning the library', async () => {
		mockLoadImages.mockRejectedValue(new Error('disk error'));

		expect(await load(event())).toMatchObject({
			images: null,
			imagesError: 'disk error',
			library: { backend: 'fs' }
		});
	});

	it('reports a library failure (and skips the config fetch) with "unknown" for non-Errors', async () => {
		mockLoadLibrary.mockRejectedValue('boom');

		expect(await load(event())).toMatchObject({
			library: null,
			libraryError: 'unknown',
			shareUrl: null,
			shuffleOn: false
		});
		expect(mockLoadConfig).not.toHaveBeenCalled();
	});
});
