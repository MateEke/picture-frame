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
		mockLoadConfig.mockResolvedValue({ library: { immich: { share_url: 'https://share' } } });
	});

	it('returns images and library with no errors on the happy path (fs backend)', async () => {
		const result = await load(event());
		expect(result).toEqual({
			images: [{ name: 'a.jpg' }],
			imagesError: undefined,
			library: { backend: 'fs' },
			libraryError: undefined,
			shareUrl: null
		});
		// fs backend never needs the config fetch.
		expect(mockLoadConfig).not.toHaveBeenCalled();
	});

	it('fetches the config and exposes the share URL only for the immich backend', async () => {
		mockLoadLibrary.mockResolvedValue({ backend: 'immich' });

		const result = await load(event());
		expect(mockLoadConfig).toHaveBeenCalledOnce();
		expect(result).toMatchObject({ shareUrl: 'https://share' });
	});

	it('falls back to a null share URL when the immich config omits it', async () => {
		mockLoadLibrary.mockResolvedValue({ backend: 'immich' });
		mockLoadConfig.mockResolvedValue(null);

		expect(await load(event())).toMatchObject({ shareUrl: null });
	});

	it('reports an images failure while still returning the library', async () => {
		mockLoadImages.mockRejectedValue(new Error('disk error'));

		expect(await load(event())).toMatchObject({
			images: null,
			imagesError: 'disk error',
			library: { backend: 'fs' }
		});
	});

	it('reports a library failure (and skips the immich config) with "unknown" for non-Errors', async () => {
		mockLoadLibrary.mockRejectedValue('boom');

		expect(await load(event())).toMatchObject({
			library: null,
			libraryError: 'unknown',
			shareUrl: null
		});
		expect(mockLoadConfig).not.toHaveBeenCalled();
	});
});
