// Helpers for the Go-style duration strings ("20m", "1h30m") used throughout the
// config API, plus the slider stop lists that drive DurationSlider.

const UNIT_SECONDS: Record<string, number> = {
	ns: 1e-9,
	us: 1e-6,
	µs: 1e-6,
	ms: 1e-3,
	s: 1,
	m: 60,
	h: 3600
};

// toSeconds parses a Go duration string to seconds; "" (and any unparseable
// input) is treated as zero, matching the server's durString("") for a zero value.
export function toSeconds(d: string): number {
	let total = 0;
	for (const [, num, unit] of (d ?? '').matchAll(/(-?\d+(?:\.\d+)?)(ns|us|µs|ms|s|m|h)/g)) {
		total += parseFloat(num) * UNIT_SECONDS[unit];
	}
	return total;
}

const FORMAT_UNITS: { secs: number; label: string }[] = [
	{ secs: 86400, label: 'day' },
	{ secs: 3600, label: 'hr' },
	{ secs: 60, label: 'min' },
	{ secs: 1, label: 'sec' }
];

// formatDuration renders a duration for humans, e.g. "2 min", "1 hr 30 min".
// A zero/blank duration renders as zeroLabel (e.g. "Never" for idle-blank).
export function formatDuration(d: string, zeroLabel = 'Off'): string {
	let remaining = Math.round(toSeconds(d));
	if (remaining <= 0) return zeroLabel;

	const parts: string[] = [];
	for (const { secs, label } of FORMAT_UNITS) {
		const n = Math.floor(remaining / secs);
		if (n > 0) {
			parts.push(`${n} ${label}`);
			remaining -= n * secs;
		}
	}
	// Two most-significant units is enough to read at a glance (e.g. "1 hr 30 min").
	return parts.slice(0, 2).join(' ');
}

// Per-field slider stops, ordered ascending. An empty string is the "off"/"never"
// stop where the field supports disabling (idle-blank, sensor reset).
export const DURATION_STOPS = {
	slideshowInterval: ['30s', '1m', '2m', '5m', '10m', '15m', '30m', '1h'],
	blankAfter: ['', '5m', '10m', '15m', '20m', '30m', '45m', '1h', '2h'],
	immichSync: ['5m', '10m', '15m', '30m', '1h', '2h', '6h', '12h', '24h'],
	weatherPoll: ['10m', '15m', '30m', '1h'],
	weatherRetry: ['10s', '30s', '1m', '2m', '5m'],
	mqttStale: ['1m', '5m', '10m', '30m', '1h'],
	sensorPoll: ['10s', '30s', '1m', '2m', '5m'],
	sensorReset: ['', '1m', '5m', '10m', '30m']
};
