import type { ConfigResponseBody, LibraryDto, SensorDto } from '$lib/api/types.gen';

// Matches the Accordion.Item values so an issue can open its section.
export type IssueSection = 'library' | 'weather' | 'mqtt' | 'sensors';

export type Issue = { section: IssueSection; message: string };

// Flat UI-ordered list for the count/jump, plus per-field maps for inline errors.
export type ValidationResult = {
	issues: Issue[];
	library: { share_url?: string; url?: string; album_ids?: string };
	weather: { lat?: string; lon?: string };
	mqtt: { broker?: string; node_id?: string; base_topic?: string; discovery_prefix?: string };
	sensors: Record<string, string>;
};

// Mirrors the Go SensorConfig.validate() type-specific required fields.
export function sensorTypeError(s: SensorDto): string | null {
	switch (s.type) {
		case 'ble':
			return bleError(s);
		case 'mqtt-subscriber':
			return mqttSubscriberError(s);
		default:
			return null; // mock has no required fields
	}
}

function bleError(s: SensorDto): string | null {
	if (!(s.mac ?? '').trim()) return 'MAC address is required.';
	const chars = s.characteristics ?? [];
	for (let i = 0; i < chars.length; i++) {
		const c = chars[i];
		if (!(c.uuid ?? '').trim()) return `Characteristic ${i + 1} needs a UUID.`;
		if (!(c.kind ?? '').trim()) return `Characteristic ${i + 1} needs a kind.`;
		if (!(c.decoder ?? '').trim()) return `Characteristic ${i + 1} needs a decoder.`;
	}
	return null;
}

function mqttSubscriberError(s: SensorDto): string | null {
	if (!(s.topic ?? '').trim()) return 'Topic is required.';
	if (!(s.kind ?? '').trim()) return 'Kind is required.';
	if (!(s.parser ?? '').trim()) return 'Parser is required.';
	return null;
}

function sensorKinds(s: SensorDto): string[] {
	const kinds: string[] = [];
	for (const c of s.characteristics ?? []) if (c.kind) kinds.push(c.kind);
	for (const r of s.mock_readings ?? []) if (r.kind) kinds.push(r.kind);
	if (s.kind) kinds.push(s.kind);
	return kinds;
}

function rangeError(v: number, lo: number, hi: number, name: string): string | null {
	if (typeof v !== 'number' || Number.isNaN(v)) return `${name} is required.`;
	if (v < lo || v > hi) return `${name} must be between ${lo} and ${hi}.`;
	return null;
}

// Keyed by sensor id for inline rendering; issue list keeps UI order.
function validateSensors(sensors: SensorDto[] | null | undefined): {
	map: Record<string, string>;
	issues: Issue[];
} {
	const map: Record<string, string> = {};
	const issues: Issue[] = [];
	const seen = new Set<string>();
	const owner = new Map<string, string>(); // "role|kind" -> sensor id
	for (const s of sensors ?? []) {
		const id = (s.id ?? '').trim();
		const msg = sensorIssue(s, id, seen, owner);
		if (!msg) continue;
		if (id) map[id] = msg;
		issues.push({ section: 'sensors', message: `Sensor ${id || '(no id)'}: ${msg}` });
	}
	return { map, issues };
}

function sensorIssue(
	s: SensorDto,
	id: string,
	seen: Set<string>,
	owner: Map<string, string>
): string | null {
	if (!id) return 'Sensor needs an ID.';
	if (seen.has(id)) return `Duplicate sensor ID "${id}".`;
	seen.add(id);
	const typeErr = sensorTypeError(s);
	if (typeErr) return typeErr;
	const roleKey = s.role || id;
	for (const k of sensorKinds(s)) {
		const key = `${roleKey}|${k}`;
		const prior = owner.get(key);
		if (prior) return `Collides with "${prior}" on role "${roleKey}", reading "${k}".`;
		owner.set(key, id);
	}
	return null;
}

// Mirrors the Go config.Validate(); issues are in accordion order.
// Mirrors the Go config.validateImmich(): exactly one connection mode must be
// configured. The share trio and the api-key block are mutually exclusive.
function validateImmich(lib: LibraryDto, result: ValidationResult) {
	const shareURL = (lib.immich.share_url ?? '').trim();
	const api = lib.immich_api_key;
	const apiConfigured = Boolean(api.url.trim() || api.api_key_set || (api.album_ids ?? []).length);

	if (!shareURL && !apiConfigured) {
		result.library.share_url = 'Choose API key and pick albums, or provide a Share URL.';
		result.issues.push({
			section: 'library',
			message: 'Library: an Immich connection is required (API key or Share URL).'
		});
		return;
	}
	if (shareURL && apiConfigured) {
		result.library.share_url =
			'Clear one connection mode: Share URL and API key are mutually exclusive.';
		result.issues.push({
			section: 'library',
			message: 'Library: Share URL and API key are mutually exclusive.'
		});
		return;
	}
	if (!apiConfigured) return;

	// The api-key trio is all-or-nothing; the API key itself is write-only, so a
	// stored key reads as api_key_set.
	if (!api.url.trim()) {
		result.library.url = 'Immich URL is required with an API key.';
		result.issues.push({ section: 'library', message: 'Library: Immich URL is required.' });
	}
	if (!api.api_key_set && !api.api_key?.trim()) {
		result.library.album_ids = 'An API key is required to list albums.';
		result.issues.push({ section: 'library', message: 'Library: Immich API key is required.' });
	}
	if (!(api.album_ids ?? []).length) {
		result.library.album_ids = 'Pick at least one album.';
		result.issues.push({ section: 'library', message: 'Library: pick at least one album.' });
	}
}

export function validate(cfg: ConfigResponseBody): ValidationResult {
	const result: ValidationResult = { issues: [], library: {}, weather: {}, mqtt: {}, sensors: {} };

	if (cfg.library.backend === 'immich') validateImmich(cfg.library, result);

	const latErr = rangeError(cfg.weather.lat, -90, 90, 'Latitude');
	if (latErr) {
		result.weather.lat = latErr;
		result.issues.push({ section: 'weather', message: `Weather: ${latErr}` });
	}
	const lonErr = rangeError(cfg.weather.lon, -180, 180, 'Longitude');
	if (lonErr) {
		result.weather.lon = lonErr;
		result.issues.push({ section: 'weather', message: `Weather: ${lonErr}` });
	}

	validateMqtt(cfg, result);

	const sensors = validateSensors(cfg.sensors);
	result.sensors = sensors.map;
	result.issues.push(...sensors.issues);

	return result;
}

function validateMqtt(cfg: ConfigResponseBody, result: ValidationResult) {
	const hasSubscriber = (cfg.sensors ?? []).some((s) => s.type === 'mqtt-subscriber');
	const bridgeEnabled = cfg.mqtt.bridge.enabled;
	if ((bridgeEnabled || hasSubscriber) && !cfg.mqtt.broker.trim()) {
		result.mqtt.broker = bridgeEnabled
			? 'Broker is required when the bridge is enabled.'
			: 'Broker is required while an MQTT sensor exists.';
		result.issues.push({ section: 'mqtt', message: 'Home Assistant: broker is required.' });
	}
	if (!bridgeEnabled) return;
	const required: [keyof ValidationResult['mqtt'], string, string][] = [
		['node_id', cfg.mqtt.bridge.node_id, 'Node ID'],
		['base_topic', cfg.mqtt.bridge.base_topic, 'Base topic'],
		['discovery_prefix', cfg.mqtt.bridge.discovery_prefix, 'Discovery prefix']
	];
	for (const [field, value, label] of required) {
		if (value.trim()) continue;
		result.mqtt[field] = `${label} is required when the bridge is enabled.`;
		result.issues.push({
			section: 'mqtt',
			message: `Home Assistant: ${label.toLowerCase()} is required.`
		});
	}
}

// Maps a backend 422 detail ("<section>: <msg>") to the section to open.
export function sectionFromDetail(detail: string): IssueSection | 'display' | null {
	const prefix = detail.split(':', 1)[0].trim().toLowerCase();
	if (prefix.startsWith('library')) return 'library';
	if (prefix.startsWith('weather')) return 'weather';
	if (prefix.startsWith('mqtt')) return 'mqtt';
	if (prefix.startsWith('sensor')) return 'sensors';
	if (prefix.startsWith('display')) return 'display';
	return null;
}
