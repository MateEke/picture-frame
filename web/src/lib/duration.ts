// Helpers for the config API's Go duration strings and DurationSlider's scales.

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

// Matches the backend's compact durString, so typed values land on equal stops.
export function toGoDuration(seconds: number): string {
	const h = Math.floor(seconds / 3600);
	const m = Math.floor((seconds % 3600) / 60);
	const s = seconds % 60;
	let out = h > 0 ? `${h}h` : '';
	if (m > 0 || (h > 0 && s > 0)) out += `${m}m`;
	if (s > 0) out += `${s}s`;
	return out;
}

export type DurationUnit = 'sec' | 'min' | 'hr';
export type DurationScale = { stops: readonly string[]; min: string; max: string };

export const DURATION_UNITS: readonly DurationUnit[] = ['sec', 'min', 'hr'];
const SECONDS_PER: Record<DurationUnit, number> = { sec: 1, min: 60, hr: 3600 };

export function isDurationUnit(v: string): v is DurationUnit {
	return DURATION_UNITS.some((u) => u === v);
}

function largestUnit(seconds: number): DurationUnit {
	if (seconds % SECONDS_PER.hr === 0) return 'hr';
	if (seconds % SECONDS_PER.min === 0) return 'min';
	return 'sec';
}

// Prefills in the largest whole unit (90m opens as "90 min"); off opens blank.
export function splitDuration(
	value: string,
	scale: DurationScale
): { amount: string; unit: DurationUnit } {
	const seconds = Math.round(toSeconds(value));
	if (seconds <= 0) return { amount: '', unit: largestUnit(toSeconds(scale.min)) };
	const unit = largestUnit(seconds);
	return { amount: String(seconds / SECONDS_PER[unit]), unit };
}

export function checkDurationParts(
	amount: string,
	unit: DurationUnit,
	scale: DurationScale
): { value: string } | { error: string } {
	const trimmed = amount.trim();
	if (!/^\d+$/.test(trimmed)) return { error: 'Enter a whole number.' };
	const seconds = Number(trimmed) * SECONDS_PER[unit];
	if (seconds === 0 && scale.stops.includes('')) return { value: '' };
	if (seconds < toSeconds(scale.min)) return { error: `Minimum is ${formatDuration(scale.min)}.` };
	if (seconds > toSeconds(scale.max)) return { error: `Maximum is ${formatDuration(scale.max)}.` };
	return { value: toGoDuration(seconds) };
}

export function durationHint(scale: DurationScale, zeroLabel: string): string {
	const range = `${formatDuration(scale.min)} to ${formatDuration(scale.max)}`;
	return scale.stops.includes('') ? `${range}, or 0 for ${zeroLabel}` : range;
}

// Deduped by duration so a saved "2m0s" doesn't become a twin of the "2m" stop.
export function sliderOptions(
	stops: readonly string[],
	value: string,
	saved: string | undefined
): string[] {
	const options = [...stops];
	for (const extra of [saved, value]) {
		if (extra === undefined) continue;
		if (!options.some((o) => toSeconds(o) === toSeconds(extra))) options.push(extra);
	}
	return options.sort((a, b) => toSeconds(a) - toSeconds(b));
}

// Ascending stops and typed-value limits per field; '' is the off position.
export const DURATIONS = {
	slideshowInterval: {
		stops: ['30s', '1m', '2m', '5m', '10m', '15m', '30m', '1h'],
		min: '5s',
		max: '24h'
	},
	blankAfter: {
		stops: ['', '5m', '10m', '15m', '20m', '30m', '45m', '1h', '2h'],
		min: '1m',
		max: '24h'
	},
	immichSync: {
		stops: ['5m', '10m', '15m', '30m', '1h', '2h', '6h', '12h', '24h'],
		min: '1m',
		max: '168h'
	},
	// A 2 min floor keeps OpenWeatherMap's free tier (1000 calls a day) safe.
	weatherPoll: { stops: ['10m', '15m', '30m', '1h'], min: '2m', max: '24h' },
	weatherRetry: { stops: ['10s', '30s', '1m', '2m', '5m'], min: '5s', max: '1h' },
	mqttStale: { stops: ['', '1m', '5m', '10m', '30m', '1h'], min: '30s', max: '24h' },
	sensorPoll: { stops: ['10s', '30s', '1m', '2m', '5m'], min: '5s', max: '1h' },
	sensorReset: { stops: ['', '1m', '5m', '10m', '30m'], min: '1m', max: '24h' }
} satisfies Record<string, DurationScale>;
