export type ConfigOptions = {
	port: number;
	imagesDir: string;
	/** bcrypt hash; gates the admin UI when set. */
	passwordHash?: string;
	/** Immich backend with a dummy (unreachable) share URL. */
	immich?: boolean;
};

// Sentinel labels the kiosk spec asserts on.
export const LABELS = {
	outside: 'E2E Outside',
	inside: 'E2E Inside',
	humidity: 'E2E Humidity'
};

/** Per-test server config: fast slideshow, mock sensors, sentinel labels, mock weather. */
export function renderConfig(opts: ConfigOptions): string {
	const auth = opts.passwordHash ? `\n[auth]\npassword_hash = "${opts.passwordHash}"\n` : '';
	// Port 9 refuses fast, so the syncer errors without an external network call.
	const library = opts.immich
		? '\n[library]\nbackend = "immich"\n\n[library.immich]\nshare_url = "http://127.0.0.1:9/share/e2e"\n'
		: '';
	return `addr = "127.0.0.1:${opts.port}"

[display]
blank_after = "20m"

[display.labels]
outside  = "${LABELS.outside}"
inside   = "${LABELS.inside}"
humidity = "${LABELS.humidity}"

[slideshow]
interval   = "2s"
images_dir = "${opts.imagesDir}"

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
