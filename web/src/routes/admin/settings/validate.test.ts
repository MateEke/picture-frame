import { describe, it, expect } from 'vitest';
import { validate, sensorTypeError, sectionFromDetail } from './validate';
import { createEmptyConfig } from './utils';
import type { SensorDto } from '$lib/api/types.gen';

describe('settings validate', () => {
	const ble = (over: Partial<SensorDto> = {}): SensorDto => ({
		id: 'b1',
		type: 'ble',
		role: 'inside',
		mac: 'AA:BB:CC:DD:EE:FF',
		characteristics: [{ uuid: 'u', kind: 'temperature', decoder: 'd' }],
		...over
	});

	it('reports no issues for a default config', () => {
		expect(validate(createEmptyConfig()).issues).toEqual([]);
	});

	describe('library', () => {
		it('requires a share URL for the immich backend', () => {
			const cfg = createEmptyConfig();
			cfg.library.backend = 'immich';
			const r = validate(cfg);
			expect(r.library.share_url).toMatch(/required/i);
			expect(r.issues).toContainEqual({ section: 'library', message: expect.any(String) });
		});

		it('accepts immich with a share URL', () => {
			const cfg = createEmptyConfig();
			cfg.library.backend = 'immich';
			cfg.library.immich.share_url = 'https://immich.example/share/x';
			expect(validate(cfg).library.share_url).toBeUndefined();
		});

		it('rejects a whitespace-only share URL (trim is load-bearing)', () => {
			const cfg = createEmptyConfig();
			cfg.library.backend = 'immich';
			cfg.library.immich.share_url = '   ';
			expect(validate(cfg).library.share_url).toMatch(/required/i);
		});

		it('does not require a share URL for the fs backend', () => {
			const cfg = createEmptyConfig();
			cfg.library.backend = 'fs';
			cfg.library.immich.share_url = '';
			expect(validate(cfg).library.share_url).toBeUndefined();
		});

		it('surfaces the immich issue with its UI message', () => {
			const cfg = createEmptyConfig();
			cfg.library.backend = 'immich';
			expect(validate(cfg).issues).toContainEqual({
				section: 'library',
				message: 'Library: Immich share URL is required.'
			});
		});
	});

	describe('weather', () => {
		it('flags out-of-range latitude and longitude', () => {
			const cfg = createEmptyConfig();
			cfg.weather.lat = 120;
			cfg.weather.lon = -200;
			const r = validate(cfg);
			expect(r.weather.lat).toMatch(/between/i);
			expect(r.weather.lon).toMatch(/between/i);
		});

		it('treats a non-numeric coordinate as required', () => {
			const cfg = createEmptyConfig();
			// A cleared number input binds to null at runtime despite the number type.
			cfg.weather.lat = null as unknown as number;
			expect(validate(cfg).weather.lat).toMatch(/required/i);
		});

		it('treats NaN as required, not out-of-range', () => {
			const cfg = createEmptyConfig();
			cfg.weather.lat = Number.NaN;
			expect(validate(cfg).weather.lat).toMatch(/required/i);
		});

		it('accepts the inclusive range bounds (±90 lat, ±180 lon)', () => {
			const cfg = createEmptyConfig();
			cfg.weather.lat = -90;
			cfg.weather.lon = 180;
			expect(validate(cfg).weather.lat).toBeUndefined();
			expect(validate(cfg).weather.lon).toBeUndefined();

			cfg.weather.lat = 90;
			cfg.weather.lon = -180;
			expect(validate(cfg).weather.lat).toBeUndefined();
			expect(validate(cfg).weather.lon).toBeUndefined();
		});

		it('rejects values just past the bounds', () => {
			const cfg = createEmptyConfig();
			cfg.weather.lat = 90.0001;
			cfg.weather.lon = -180.0001;
			expect(validate(cfg).weather.lat).toMatch(/between -90 and 90/i);
			expect(validate(cfg).weather.lon).toMatch(/between -180 and 180/i);
		});

		it('raises a weather issue for an out-of-range longitude', () => {
			const cfg = createEmptyConfig();
			cfg.weather.lon = -200;
			expect(validate(cfg).issues).toContainEqual({
				section: 'weather',
				message: 'Weather: Longitude must be between -180 and 180.'
			});
		});

		it('raises a weather issue for an out-of-range latitude', () => {
			const cfg = createEmptyConfig();
			cfg.weather.lat = 200;
			expect(validate(cfg).issues).toContainEqual({
				section: 'weather',
				message: 'Weather: Latitude must be between -90 and 90.'
			});
		});
	});

	describe('mqtt', () => {
		it('requires a broker when the bridge is enabled', () => {
			const cfg = createEmptyConfig();
			cfg.mqtt.bridge.enabled = true;
			expect(validate(cfg).mqtt.broker).toMatch(/bridge/i);
		});

		it('requires a broker when an mqtt-subscriber sensor exists', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [
				{
					id: 's',
					type: 'mqtt-subscriber',
					role: 'r',
					topic: 't',
					kind: 'temperature',
					parser: 'p'
				}
			];
			expect(validate(cfg).mqtt.broker).toMatch(/sensor/i);
		});

		it('distinguishes the broker message for bridge vs sensor', () => {
			const bridgeCfg = createEmptyConfig();
			bridgeCfg.mqtt.bridge.enabled = true;
			expect(validate(bridgeCfg).mqtt.broker).toBe(
				'Broker is required when the bridge is enabled.'
			);

			const sensorCfg = createEmptyConfig();
			sensorCfg.sensors = [
				{
					id: 's',
					type: 'mqtt-subscriber',
					role: 'r',
					topic: 't',
					kind: 'temperature',
					parser: 'p'
				}
			];
			expect(validate(sensorCfg).mqtt.broker).toBe(
				'Broker is required while an MQTT sensor exists.'
			);
		});

		it('treats a whitespace-only broker as missing', () => {
			const cfg = createEmptyConfig();
			cfg.mqtt.bridge.enabled = true;
			cfg.mqtt.broker = '   ';
			expect(validate(cfg).mqtt.broker).toMatch(/required/i);
		});

		it('does not require bridge identifiers when the bridge is disabled', () => {
			const cfg = createEmptyConfig();
			cfg.mqtt.bridge.enabled = false;
			cfg.mqtt.bridge.node_id = '';
			cfg.mqtt.bridge.base_topic = '';
			cfg.mqtt.bridge.discovery_prefix = '';
			const r = validate(cfg);
			expect(r.mqtt.node_id).toBeUndefined();
			expect(r.mqtt.base_topic).toBeUndefined();
			expect(r.mqtt.discovery_prefix).toBeUndefined();
		});

		it('requires bridge identifiers when enabled', () => {
			const cfg = createEmptyConfig();
			cfg.mqtt.broker = 'tcp://host:1883';
			cfg.mqtt.bridge.enabled = true;
			cfg.mqtt.bridge.node_id = '';
			cfg.mqtt.bridge.base_topic = '';
			cfg.mqtt.bridge.discovery_prefix = '';
			const r = validate(cfg);
			expect(r.mqtt.node_id).toBeTruthy();
			expect(r.mqtt.base_topic).toBeTruthy();
			expect(r.mqtt.discovery_prefix).toBeTruthy();
		});

		it('treats whitespace-only bridge identifiers as missing', () => {
			const cfg = createEmptyConfig();
			cfg.mqtt.broker = 'tcp://host:1883';
			cfg.mqtt.bridge.enabled = true;
			cfg.mqtt.bridge.node_id = '   ';
			cfg.mqtt.bridge.base_topic = '\t';
			cfg.mqtt.bridge.discovery_prefix = ' ';
			const r = validate(cfg);
			expect(r.mqtt.node_id).toMatch(/required/i);
			expect(r.mqtt.base_topic).toMatch(/required/i);
			expect(r.mqtt.discovery_prefix).toMatch(/required/i);
		});

		it('raises the broker issue with its UI message', () => {
			const cfg = createEmptyConfig();
			cfg.mqtt.bridge.enabled = true;
			expect(validate(cfg).issues).toContainEqual({
				section: 'mqtt',
				message: 'Home Assistant: broker is required.'
			});
		});

		it('raises a lower-cased issue per missing bridge identifier', () => {
			const cfg = createEmptyConfig();
			cfg.mqtt.broker = 'tcp://host:1883';
			cfg.mqtt.bridge.enabled = true;
			cfg.mqtt.bridge.node_id = '';
			cfg.mqtt.bridge.base_topic = '';
			cfg.mqtt.bridge.discovery_prefix = '';
			const issues = validate(cfg).issues;
			expect(issues).toContainEqual({
				section: 'mqtt',
				message: 'Home Assistant: node id is required.'
			});
			expect(issues).toContainEqual({
				section: 'mqtt',
				message: 'Home Assistant: base topic is required.'
			});
			expect(issues).toContainEqual({
				section: 'mqtt',
				message: 'Home Assistant: discovery prefix is required.'
			});
			// The inline field errors keep their original-case label.
			expect(validate(cfg).mqtt.node_id).toBe('Node ID is required when the bridge is enabled.');
		});
	});

	describe('sensors', () => {
		it('flags a ble sensor missing its mac', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ mac: '' })];
			expect(validate(cfg).sensors.b1).toMatch(/mac/i);
		});

		it('flags an incomplete characteristic', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ characteristics: [{ uuid: '', kind: 'temperature', decoder: 'd' }] })];
			expect(validate(cfg).sensors.b1).toMatch(/uuid/i);
		});

		it('flags a duplicate sensor id', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ id: 'dup', role: 'a' }), ble({ id: 'dup', role: 'b' })];
			expect(validate(cfg).sensors.dup).toMatch(/duplicate/i);
		});

		it('flags a role/kind collision between two sensors', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ id: 'x', role: 'inside' }), ble({ id: 'y', role: 'inside' })];
			expect(validate(cfg).sensors.y).toMatch(/collide/i);
		});

		it('accepts the same kind under distinct roles', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ id: 'x', role: 'inside' }), ble({ id: 'y', role: 'outside' })];
			expect(validate(cfg).issues).toEqual([]);
		});

		it('flags a sensor with no id, labelling the issue "(no id)"', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ id: '' })];
			const r = validate(cfg);
			expect(r.issues).toContainEqual({
				section: 'sensors',
				message: 'Sensor (no id): Sensor needs an ID.'
			});
			// No id means no inline (id-keyed) entry, only the issue.
			expect(r.sensors).toEqual({});
		});

		it('treats a whitespace-only id as missing', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ id: '   ' })];
			expect(validate(cfg).issues[0].message).toMatch(/needs an ID/i);
		});

		it('flags a characteristic missing its kind', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ characteristics: [{ uuid: 'u', kind: '', decoder: 'd' }] })];
			expect(validate(cfg).sensors.b1).toMatch(/kind/i);
		});

		it('flags a characteristic missing its decoder', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ characteristics: [{ uuid: 'u', kind: 'temperature', decoder: '' }] })];
			expect(validate(cfg).sensors.b1).toMatch(/decoder/i);
		});

		it('numbers the offending characteristic (1-based)', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [
				ble({
					characteristics: [
						{ uuid: 'u1', kind: 'temperature', decoder: 'd' },
						{ uuid: '', kind: 'humidity', decoder: 'd' }
					]
				})
			];
			expect(validate(cfg).sensors.b1).toBe('Characteristic 2 needs a UUID.');
		});

		it('detects a collision via top-level kind', () => {
			const cfg = createEmptyConfig();
			const sub = (id: string): SensorDto => ({
				id,
				type: 'mqtt-subscriber',
				role: 'inside',
				topic: `t/${id}`,
				kind: 'temperature',
				parser: 'p'
			});
			cfg.mqtt.broker = 'tcp://host:1883';
			cfg.sensors = [sub('a'), sub('b')];
			expect(validate(cfg).sensors.b).toMatch(/collides/i);
		});

		it('detects a collision via mock readings, naming the role and reading', () => {
			const cfg = createEmptyConfig();
			const mock = (id: string): SensorDto => ({
				id,
				type: 'mock',
				role: 'inside',
				mock_readings: [{ kind: 'temperature', value: 1, delta: 0 }]
			});
			cfg.sensors = [mock('a'), mock('b')];
			expect(validate(cfg).sensors.b).toBe(
				'Collides with "a" on role "inside", reading "temperature".'
			);
		});

		it('does not false-collide on empty-kind mock readings', () => {
			const cfg = createEmptyConfig();
			const mock = (id: string): SensorDto => ({
				id,
				type: 'mock',
				role: 'inside',
				mock_readings: [{ kind: '', value: 1, delta: 0 }]
			});
			cfg.sensors = [mock('a'), mock('b')];
			expect(validate(cfg).issues).toEqual([]);
		});

		it('does not false-collide on sensors that publish no kinds', () => {
			const cfg = createEmptyConfig();
			const mock = (id: string): SensorDto => ({ id, type: 'mock', role: 'inside' });
			cfg.sensors = [mock('a'), mock('b')];
			expect(validate(cfg).issues).toEqual([]);
		});

		it('keys the collision by role, not sensor id (distinct ids, shared role)', () => {
			const cfg = createEmptyConfig();
			cfg.sensors = [ble({ id: 'x', role: 'inside' }), ble({ id: 'y', role: 'inside' })];
			expect(validate(cfg).sensors.y).toBe(
				'Collides with "x" on role "inside", reading "temperature".'
			);
		});
	});

	describe('issue ordering', () => {
		it('lists issues in accordion (UI) order: library, weather, mqtt, sensors', () => {
			const cfg = createEmptyConfig();
			cfg.library.backend = 'immich';
			cfg.weather.lat = 200;
			cfg.mqtt.bridge.enabled = true;
			cfg.sensors = [ble({ mac: '' })];
			const sections = validate(cfg).issues.map((i) => i.section);
			expect(sections).toEqual(['library', 'weather', 'mqtt', 'sensors']);
		});
	});

	describe('sensorTypeError', () => {
		it('returns null for a complete mock sensor', () => {
			expect(sensorTypeError({ id: 'm', type: 'mock', role: 'r' })).toBeNull();
		});

		it('requires the mqtt-subscriber fields in order', () => {
			expect(sensorTypeError({ id: 's', type: 'mqtt-subscriber', role: 'r' })).toMatch(/topic/i);
			expect(sensorTypeError({ id: 's', type: 'mqtt-subscriber', role: 'r', topic: 't' })).toMatch(
				/kind/i
			);
			expect(
				sensorTypeError({ id: 's', type: 'mqtt-subscriber', role: 'r', topic: 't', kind: 'k' })
			).toMatch(/parser/i);
		});

		it('accepts a complete mqtt-subscriber sensor', () => {
			expect(
				sensorTypeError({
					id: 's',
					type: 'mqtt-subscriber',
					role: 'r',
					topic: 't',
					kind: 'k',
					parser: 'p'
				})
			).toBeNull();
		});

		it('reports ble required fields in order: mac, then per-characteristic', () => {
			expect(sensorTypeError({ id: 'b', type: 'ble', role: 'r' })).toMatch(/mac/i);
			expect(
				sensorTypeError({
					id: 'b',
					type: 'ble',
					role: 'r',
					mac: 'AA:BB:CC:DD:EE:FF',
					characteristics: [{ uuid: 'u', kind: 'temperature', decoder: '' }]
				})
			).toMatch(/decoder/i);
		});

		it('treats a whitespace-only mac as missing', () => {
			expect(sensorTypeError({ id: 'b', type: 'ble', role: 'r', mac: '   ' })).toMatch(/mac/i);
		});

		it('treats whitespace-only characteristic fields as missing', () => {
			const withChar = (c: { uuid: string; kind: string; decoder: string }) =>
				sensorTypeError({
					id: 'b',
					type: 'ble',
					role: 'r',
					mac: 'AA:BB:CC:DD:EE:FF',
					characteristics: [c]
				});
			expect(withChar({ uuid: '  ', kind: 'k', decoder: 'd' })).toMatch(/UUID/i);
			expect(withChar({ uuid: 'u', kind: '\t', decoder: 'd' })).toMatch(/kind/i);
			expect(withChar({ uuid: 'u', kind: 'k', decoder: ' ' })).toMatch(/decoder/i);
		});

		it('numbers the characteristic in kind and decoder messages too', () => {
			const second = (c: { uuid: string; kind: string; decoder: string }) =>
				sensorTypeError({
					id: 'b',
					type: 'ble',
					role: 'r',
					mac: 'AA:BB:CC:DD:EE:FF',
					characteristics: [{ uuid: 'u1', kind: 'temperature', decoder: 'd' }, c]
				});
			expect(second({ uuid: 'u2', kind: '', decoder: 'd' })).toBe('Characteristic 2 needs a kind.');
			expect(second({ uuid: 'u2', kind: 'humidity', decoder: '' })).toBe(
				'Characteristic 2 needs a decoder.'
			);
		});

		it('treats whitespace-only mqtt-subscriber fields as missing', () => {
			const sub = (over: Partial<SensorDto>) =>
				sensorTypeError({ id: 's', type: 'mqtt-subscriber', role: 'r', ...over });
			expect(sub({ topic: '  ' })).toMatch(/topic/i);
			expect(sub({ topic: 't', kind: '  ' })).toMatch(/kind/i);
			expect(sub({ topic: 't', kind: 'k', parser: '  ' })).toMatch(/parser/i);
		});
	});

	describe('sectionFromDetail', () => {
		it('maps a backend detail prefix to its accordion section', () => {
			expect(sectionFromDetail('library: immich.share_url required')).toBe('library');
			expect(sectionFromDetail('mqtt: broker required')).toBe('mqtt');
			expect(sectionFromDetail('weather: invalid units')).toBe('weather');
			expect(sectionFromDetail('sensor "x": ble sensor missing mac')).toBe('sensors');
			expect(sectionFromDetail('display: unknown backend')).toBe('display');
		});

		it('returns null for an unrecognised detail', () => {
			expect(sectionFromDetail('something unexpected')).toBeNull();
		});

		it('matches case-insensitively and trims the prefix', () => {
			expect(sectionFromDetail('LIBRARY: oops')).toBe('library');
			expect(sectionFromDetail('  Weather : bad')).toBe('weather');
		});

		it('matches on prefix, so a longer word starting with the section still maps', () => {
			expect(sectionFromDetail('sensors[2]: missing mac')).toBe('sensors');
			expect(sectionFromDetail('library_backend: x')).toBe('library');
			expect(sectionFromDetail('weather-poll: x')).toBe('weather');
			expect(sectionFromDetail('mqtt_bridge: x')).toBe('mqtt');
			expect(sectionFromDetail('display_output: x')).toBe('display');
		});

		it('uses only the text before the first colon', () => {
			// "weather" appears after the colon; the prefix is "display" → display.
			expect(sectionFromDetail('display: weather widget broken')).toBe('display');
		});
	});
});
