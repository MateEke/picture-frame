import type { ConfigMetaBody, ConfigResponseBody, SensorDto } from '$lib/api/types.gen';

// structural equality for draft-vs-saved dirty checks
export const eq = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

// hasMotionSensor reports whether any configured sensor publishes a motion kind.
// Idle-blank only auto-wakes on a motion event, so without one the screen blanks
// and never wakes, the UI disables blank_after in that case.
export function hasMotionSensor(sensors: SensorDto[] | null | undefined): boolean {
	return (sensors ?? []).some((s) => {
		if (s.kind === 'motion') return true;
		if (s.characteristics?.some((c) => c.kind === 'motion')) return true;
		return s.mock_readings?.some((r) => r.kind === 'motion') ?? false;
	});
}

export function createEmptyConfig(): ConfigResponseBody {
	return {
		log_level: 'info',
		bluetooth_adapter: 'hci0',
		display: {
			blank_after: '20m',
			backend: 'wlopm',
			output: 'HDMI-A-1',
			locale: 'en-US',
			hide_clock_date: false,
			timezone: '',
			labels: { outside: '', inside: '', humidity: '' }
		},
		slideshow: { interval: '2m', randomize: false, split_screen: true, images_dir: 'images' },
		library: {
			backend: 'fs',
			immich: { share_url: '', share_password: '', share_password_set: false, sync_interval: '15m' }
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
	};
}

// Empty option lists for the settings <select>s, used until /api/config/meta loads.
export function createEmptyMeta(): ConfigMetaBody {
	return {
		decoders: [],
		kinds: [],
		units: [],
		backends: [],
		sensor_types: [],
		address_types: [],
		log_levels: []
	};
}

// Strips server-derived fields before dirty comparison.
export function configPayload(c: ConfigResponseBody): Partial<ConfigResponseBody> {
	const temp: Partial<ConfigResponseBody> = { ...c };
	delete temp.restart_pending;
	return temp;
}
