import { describe, it, expect, vi, beforeEach } from 'vitest';
import { load } from './+page';

const mockLoadScreen = vi.fn();
const mockLoadConfig = vi.fn();
const mockLoadWiFiStatus = vi.fn();
const mockLoadSystemInfo = vi.fn();
const mockLoadLibrary = vi.fn();
const mockLoadUpdate = vi.fn();

vi.mock('$lib/screen', () => ({ loadScreen: (...a: unknown[]) => mockLoadScreen(...a) }));
vi.mock('$lib/config', () => ({ loadConfig: (...a: unknown[]) => mockLoadConfig(...a) }));
vi.mock('$lib/wifi', () => ({ loadWiFiStatus: (...a: unknown[]) => mockLoadWiFiStatus(...a) }));
vi.mock('$lib/system', () => ({ loadSystemInfo: (...a: unknown[]) => mockLoadSystemInfo(...a) }));
vi.mock('$lib/library', () => ({ loadLibrary: (...a: unknown[]) => mockLoadLibrary(...a) }));
vi.mock('$lib/updater', () => ({ loadUpdate: (...a: unknown[]) => mockLoadUpdate(...a) }));

// The loader only reads `fetch`; cast the minimal event once.
function event() {
	return { fetch: vi.fn() } as unknown as Parameters<typeof load>[0];
}

describe('admin dashboard load', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		mockLoadScreen.mockResolvedValue({ state: 'on', auto: true });
		mockLoadConfig.mockResolvedValue({ log_level: 'info' });
		mockLoadWiFiStatus.mockResolvedValue({ mode: 'connected' });
		mockLoadSystemInfo.mockResolvedValue({ version: '1.0' });
		mockLoadLibrary.mockResolvedValue({ backend: 'fs' });
		mockLoadUpdate.mockResolvedValue({ current: '1.0', available: false });
	});

	it('aggregates every tile on the happy path with no screen error', async () => {
		const result = await load(event());
		expect(result).toEqual({
			screen: { state: 'on', auto: true },
			config: { log_level: 'info' },
			wifi: { mode: 'connected' },
			system: { version: '1.0' },
			library: { backend: 'fs' },
			update: { current: '1.0', available: false }
		});
	});

	it('surfaces a screen error inline while keeping the other tiles', async () => {
		mockLoadScreen.mockRejectedValue(new Error('screen offline'));

		const result = await load(event());
		expect(result).toMatchObject({
			screen: null,
			screenError: 'screen offline',
			config: { log_level: 'info' },
			system: { version: '1.0' }
		});
	});

	it('uses "unknown" for a non-Error screen failure', async () => {
		mockLoadScreen.mockRejectedValue('boom');

		expect(await load(event())).toMatchObject({ screen: null, screenError: 'unknown' });
	});

	it('degrades wifi and library to null when they reject, without failing the page', async () => {
		mockLoadWiFiStatus.mockRejectedValue(new Error('no wifi'));
		mockLoadLibrary.mockRejectedValue(new Error('no library'));

		const result = await load(event());
		expect(result).toMatchObject({ wifi: null, library: null, config: { log_level: 'info' } });
		expect(result).not.toHaveProperty('screenError');
	});
});
