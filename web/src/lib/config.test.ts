import { describe, it, expect, vi, afterEach } from 'vitest';
import { toaster } from './toaster';
import { loadConfig, loadConfigMeta, saveConfig, restartFrame } from './config';

vi.mock('./toaster', () => ({
	toaster: { error: vi.fn(), success: vi.fn() }
}));

vi.mock('$app/navigation', () => ({
	invalidate: vi.fn().mockResolvedValue(undefined)
}));

const mockApiGetConfig = vi.fn();
const mockApiGetConfigMeta = vi.fn();
const mockApiPutConfig = vi.fn();
const mockApiSystemRestart = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiGetConfig: (...args: unknown[]) => mockApiGetConfig(...args),
	apiGetConfigMeta: (...args: unknown[]) => mockApiGetConfigMeta(...args),
	apiPutConfig: (...args: unknown[]) => mockApiPutConfig(...args),
	apiSystemRestart: (...args: unknown[]) => mockApiSystemRestart(...args)
}));

const sampleConfig = {
	log_level: 'info' as const,
	bluetooth_adapter: 'hci0',
	display: {
		blank_after: '20m0s',
		backend: 'wlopm' as const,
		output: 'HDMI-A-1',
		locale: 'en-US',
		labels: { outside: '', inside: '', humidity: '' }
	},
	slideshow: { interval: '2m0s', randomize: false, split_screen: true, images_dir: 'images' },
	library: {
		backend: 'fs' as const,
		immich: {
			share_url: '',
			share_password_set: false,
			sync_interval: ''
		}
	},
	sensors: null,
	weather: {
		api_key_set: false,
		lat: 0,
		lon: 0,
		poll_interval: '10m0s',
		retry_interval: '30s',
		units: 'metric' as const
	},
	mqtt: {
		broker: '',
		username: '',
		password_set: false,
		client_id: 'frame',
		bridge: {
			enabled: false,
			node_id: 'pf',
			base_topic: 'picture-frame',
			discovery_prefix: 'homeassistant',
			stale_after: '10m0s'
		}
	},
	updater: { auto_update: false, update_hour: 2, github_repo: '', github_token_set: false },
	restart_pending: false
};

describe('config', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('loadConfig', () => {
		it('returns config data on success', async () => {
			mockApiGetConfig.mockResolvedValue({ data: sampleConfig, error: undefined });
			const result = await loadConfig(fetch);
			expect(result).toMatchObject({ bluetooth_adapter: 'hci0' });
			expect(toaster.error).not.toHaveBeenCalled();
		});

		it('toasts and returns null on error', async () => {
			mockApiGetConfig.mockResolvedValue({ data: undefined, error: { detail: 'internal error' } });
			const result = await loadConfig(fetch);
			expect(result).toBeNull();
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Failed to load config',
				description: 'Server returned an error'
			});
		});

		it('toasts and returns null on thrown error, using the error message', async () => {
			mockApiGetConfig.mockRejectedValue(new Error('network error'));
			const result = await loadConfig(fetch);
			expect(result).toBeNull();
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Failed to load config',
				description: 'network error'
			});
		});

		it('falls back to "Unknown error" for a non-Error rejection', async () => {
			mockApiGetConfig.mockRejectedValue('boom');
			await loadConfig(fetch);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Failed to load config',
				description: 'Unknown error'
			});
		});
	});

	describe('loadConfigMeta', () => {
		const meta = {
			decoders: ['raw_float', 'raw_int'],
			kinds: ['temperature', 'humidity', 'motion'],
			units: ['standard', 'metric', 'imperial'],
			backends: ['fs', 'immich'],
			sensor_types: ['ble', 'mqtt-subscriber', 'mock'],
			address_types: ['random', 'public']
		};

		it('returns meta on success', async () => {
			mockApiGetConfigMeta.mockResolvedValue({ data: meta, error: undefined });
			const result = await loadConfigMeta(fetch);
			expect(result?.decoders).toContain('raw_float');
		});

		it('toasts and returns null on error', async () => {
			mockApiGetConfigMeta.mockResolvedValue({ data: undefined, error: {} });
			const result = await loadConfigMeta(fetch);
			expect(result).toBeNull();
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Failed to load config meta',
				description: 'Server returned an error'
			});
		});

		it('toasts the error message on a thrown error, "Unknown error" otherwise', async () => {
			mockApiGetConfigMeta.mockRejectedValue(new Error('boom'));
			await loadConfigMeta(fetch);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Failed to load config meta',
				description: 'boom'
			});

			vi.clearAllMocks();
			mockApiGetConfigMeta.mockRejectedValue(42);
			await loadConfigMeta(fetch);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Failed to load config meta',
				description: 'Unknown error'
			});
		});
	});

	describe('saveConfig', () => {
		it('returns ok with restart_pending on success', async () => {
			mockApiPutConfig.mockResolvedValue({ data: { restart_pending: true }, error: undefined });
			const result = await saveConfig(sampleConfig);
			expect(result).toMatchObject({ ok: true, restart_pending: true });
			expect(toaster.error).not.toHaveBeenCalled();
		});

		it('returns ok:false with detail on server error', async () => {
			mockApiPutConfig.mockResolvedValue({
				data: undefined,
				error: { detail: 'invalid sensor id' }
			});
			const result = await saveConfig(sampleConfig);
			expect(result).toMatchObject({ ok: false, detail: 'invalid sensor id' });
		});

		it('falls back to a generic detail when the error omits one', async () => {
			mockApiPutConfig.mockResolvedValue({ data: undefined, error: {} });
			const result = await saveConfig(sampleConfig);
			expect(result).toEqual({ ok: false, detail: 'Server returned an error' });
		});

		it('defaults restart_pending to false when the response omits it', async () => {
			mockApiPutConfig.mockResolvedValue({ data: undefined, error: undefined });
			const result = await saveConfig(sampleConfig);
			expect(result).toEqual({ ok: true, restart_pending: false });
		});

		it('strips restart_pending before sending', async () => {
			mockApiPutConfig.mockResolvedValue({ data: { restart_pending: false }, error: undefined });
			await saveConfig({ ...sampleConfig, $schema: 'http://example.com', restart_pending: true });
			const sentBody = mockApiPutConfig.mock.calls[0][0].body;
			expect(sentBody).not.toHaveProperty('restart_pending');
		});

		it('toasts and returns ok:false on thrown error', async () => {
			mockApiPutConfig.mockRejectedValue(new Error('timeout'));
			const result = await saveConfig(sampleConfig);
			expect(result).toEqual({ ok: false, detail: 'timeout' });
			expect(toaster.error).toHaveBeenCalledWith({ title: 'Save failed', description: 'timeout' });
		});

		it('uses "Unknown error" detail for a non-Error throw', async () => {
			mockApiPutConfig.mockRejectedValue('boom');
			const result = await saveConfig(sampleConfig);
			expect(result).toEqual({ ok: false, detail: 'Unknown error' });
		});
	});

	describe('restartFrame', () => {
		it('returns true on success', async () => {
			mockApiSystemRestart.mockResolvedValue({ error: undefined });
			const result = await restartFrame();
			expect(result).toBe(true);
		});

		it('returns false and toasts on error', async () => {
			mockApiSystemRestart.mockResolvedValue({ error: { detail: 'not configured' } });
			const result = await restartFrame();
			expect(result).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Restart failed',
				description: 'Server returned an error'
			});
		});

		it('returns false and toasts on thrown error (message or "Unknown error")', async () => {
			mockApiSystemRestart.mockRejectedValue(new Error('network'));
			expect(await restartFrame()).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Restart failed',
				description: 'network'
			});

			vi.clearAllMocks();
			mockApiSystemRestart.mockRejectedValue(null);
			expect(await restartFrame()).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Restart failed',
				description: 'Unknown error'
			});
		});
	});
});
