export type ConfigOptions = {
	port: number;
	imagesDir: string;
	/** bcrypt hash; gates the admin UI when set. */
	passwordHash?: string;
	/** Immich backend with a dummy (unreachable) share URL. */
	immich?: boolean;
	/** Hide the clock and date on the kiosk. */
	hideClockDate?: boolean;
	/** IANA timezone for the kiosk clock/date; empty follows the device. */
	timezone?: string;
	/** Slideshow dwell; defaults to the fast 2s the other specs rely on. */
	slideshowInterval?: string;
};

/** Per-test server config: fast slideshow, mock sensors, mock weather. */
export function renderConfig(opts: ConfigOptions): string {
	const auth = opts.passwordHash ? `\n[auth]\npassword_hash = "${opts.passwordHash}"\n` : '';
	// Port 9 refuses fast, so the syncer errors without an external network call.
	const library = opts.immich
		? '\n[library]\nbackend = "immich"\n\n[library.immich]\nshare_url = "http://127.0.0.1:9/share/e2e"\n'
		: '';
	const displayExtra =
		(opts.hideClockDate ? 'hide_clock_date = true\n' : '') +
		(opts.timezone ? `timezone = "${opts.timezone}"\n` : '');
	// Sentinel labels the settings spec asserts on. They are still a configurable
	// feature even though the kiosk overlay no longer shows weather readings.
	const display = `[display]
blank_after = "20m"
${displayExtra}
[display.labels]
outside  = "E2E Outside"
inside   = "E2E Inside"
humidity = "E2E Humidity"`;
	const slideshow = `[slideshow]
interval   = "${opts.slideshowInterval ?? '2s'}"
images_dir = "${opts.imagesDir}"`;

	return `addr = "127.0.0.1:${opts.port}"

${display}
${slideshow}

[[sensor]]
id            = "mock_inside"
type          = "mock"
role          = "inside"
poll_interval = "1s"

[[sensor.mock_reading]]
kind  = "temperature"
value = 22.0
delta = 0.5

[[sensor.mock_reading]]
kind  = "humidity"
value = 48.0
delta = -1.0

[[sensor.mock_reading]]
kind  = "motion"
value = 1.0

[[sensor]]
id            = "mock_outside"
type          = "mock"
role          = "outside"
poll_interval = "1s"

[[sensor.mock_reading]]
kind  = "temperature"
value = 5.0
delta = 0.0

[weather]
api_key        = ""
lat            = 47.3567
lon            = 19.4485
poll_interval  = "10m"
retry_interval = "30s"
units          = "metric"
${library}${auth}`;
}
