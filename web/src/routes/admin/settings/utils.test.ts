import { describe, it, expect } from 'vitest';
import { eq, hasMotionSensor, createEmptyConfig, createEmptyMeta, configPayload } from './utils';
import type { SensorDto } from '$lib/api/types.gen';

describe('settings utils', () => {
	describe('eq', () => {
		it('is true for structurally equal but distinct objects', () => {
			expect(eq({ a: 1, b: [2] }, { a: 1, b: [2] })).toBe(true);
		});

		it('is false when contents differ', () => {
			expect(eq({ a: 1 }, { a: 2 })).toBe(false);
			expect(eq([1, 2], [1, 2, 3])).toBe(false);
		});

		it('compares by value, not key order is irrelevant only as JSON allows', () => {
			// JSON.stringify is order-sensitive, so differently-ordered keys are unequal.
			expect(eq({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(false);
		});
	});

	describe('hasMotionSensor', () => {
		const base: SensorDto = { id: 's', type: 'mock', role: '' };

		it('returns false for nullish input', () => {
			expect(hasMotionSensor(null)).toBe(false);
			expect(hasMotionSensor(undefined)).toBe(false);
		});

		it('detects an mqtt-subscriber motion sensor by kind', () => {
			expect(hasMotionSensor([{ ...base, type: 'mqtt-subscriber', kind: 'motion' }])).toBe(true);
		});

		it('detects motion from a BLE characteristic', () => {
			expect(
				hasMotionSensor([
					{ ...base, type: 'ble', characteristics: [{ uuid: 'u', kind: 'motion', decoder: 'd' }] }
				])
			).toBe(true);
		});

		it('ignores a non-motion BLE characteristic', () => {
			expect(
				hasMotionSensor([
					{
						...base,
						type: 'ble',
						characteristics: [{ uuid: 'u', kind: 'temperature', decoder: 'd' }]
					}
				])
			).toBe(false);
		});

		it('detects motion from a mock reading', () => {
			expect(
				hasMotionSensor([{ ...base, mock_readings: [{ kind: 'motion', value: 1, delta: 0 }] }])
			).toBe(true);
		});

		it('ignores a non-motion mock reading', () => {
			expect(
				hasMotionSensor([{ ...base, mock_readings: [{ kind: 'temperature', value: 1, delta: 0 }] }])
			).toBe(false);
		});

		it('returns false when no sensor reports motion', () => {
			expect(hasMotionSensor([{ ...base, type: 'mqtt-subscriber', kind: 'temperature' }])).toBe(
				false
			);
		});

		it('detects motion in any sensor of a mixed list', () => {
			expect(
				hasMotionSensor([
					{ ...base, kind: 'temperature' },
					{ ...base, id: 's2', kind: 'motion' }
				])
			).toBe(true);
		});

		it('detects motion among mixed characteristics (some, not every)', () => {
			expect(
				hasMotionSensor([
					{
						...base,
						type: 'ble',
						characteristics: [
							{ uuid: 'u1', kind: 'temperature', decoder: 'd' },
							{ uuid: 'u2', kind: 'motion', decoder: 'd' }
						]
					}
				])
			).toBe(true);
		});

		it('detects motion among mixed mock readings (some, not every)', () => {
			expect(
				hasMotionSensor([
					{
						...base,
						mock_readings: [
							{ kind: 'temperature', value: 1, delta: 0 },
							{ kind: 'motion', value: 1, delta: 0 }
						]
					}
				])
			).toBe(true);
		});
	});

	describe('createEmptyConfig', () => {
		it('produces the documented default draft config', () => {
			expect(createEmptyConfig()).toEqual({
				log_level: 'info',
				bluetooth_adapter: 'hci0',
				display: {
					blank_after: '20m',
					backend: 'wlopm',
					output: 'HDMI-A-1',
					locale: 'en-US',
					labels: { outside: '', inside: '', humidity: '' }
				},
				slideshow: { interval: '2m', randomize: false, images_dir: 'images' },
				library: {
					backend: 'fs',
					immich: {
						share_url: '',
						share_password: '',
						share_password_set: false,
						sync_interval: '15m'
					}
				},
				sensors: [],
				weather: {
					api_key: '',
					api_key_set: false,
					lat: 0,
					lon: 0,
					poll_interval: '10m',
					retry_interval: '30s',
					units: 'metric'
				},
				mqtt: {
					broker: '',
					username: '',
					password: '',
					password_set: false,
					client_id: 'picture-frame',
					bridge: {
						enabled: false,
						node_id: 'picture_frame',
						base_topic: 'picture-frame',
						discovery_prefix: 'homeassistant',
						stale_after: '10m'
					}
				},
				updater: {
					auto_update: false,
					update_hour: 2,
					github_repo: '',
					github_token: '',
					github_token_set: false
				},
				restart_pending: false
			});
		});

		it('returns a fresh object each call (draft and saved must not alias)', () => {
			expect(createEmptyConfig()).not.toBe(createEmptyConfig());
			expect(createEmptyConfig().display).not.toBe(createEmptyConfig().display);
			expect(createEmptyConfig().display.labels).not.toBe(createEmptyConfig().display.labels);
		});
	});

	describe('createEmptyMeta', () => {
		it('returns empty option lists for every select', () => {
			expect(createEmptyMeta()).toEqual({
				decoders: [],
				kinds: [],
				units: [],
				backends: [],
				sensor_types: [],
				address_types: [],
				log_levels: []
			});
		});
	});

	describe('configPayload', () => {
		it('strips restart_pending', () => {
			const payload = configPayload({ ...createEmptyConfig(), restart_pending: true });
			expect('restart_pending' in payload).toBe(false);
		});

		it('preserves all other fields', () => {
			const cfg = createEmptyConfig();
			cfg.log_level = 'debug';
			const payload = configPayload(cfg);
			expect(payload.log_level).toBe('debug');
			expect(payload.display).toEqual(cfg.display);
		});

		it('does not mutate the input config', () => {
			const cfg = createEmptyConfig();
			configPayload(cfg);
			expect(cfg.restart_pending).toBe(false);
		});
	});
});
