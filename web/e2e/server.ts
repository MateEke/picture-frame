import { spawn, type ChildProcess } from 'node:child_process';
import { cp, mkdtemp, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';
import os from 'node:os';
import { renderConfig } from './config-template';
import { binaryPath, freePort, seedImagesDir } from './helpers';

export type ServerOptions = {
	/** bcrypt hash; boots password-gated when set. */
	passwordHash?: string;
	/** WIFI_MOCK=off → wifi routes serve 503. */
	wifiOff?: boolean;
	/** Immich library backend (dummy share URL; syncer fails in the background). */
	immich?: boolean;
	/** Updater mock: fake newer version offered (unset = up to date). */
	updateLatest?: string;
	/** Updater mock: terminal result after an apply (e.g. "rolled back from v9.9.9"). */
	updateOutcome?: string;
	/** Updater mock: simulate an unreachable release source. */
	updateOffline?: boolean;
	/** Seed image directory to copy in; defaults to the shared fixtures/images. */
	seedDir?: string;
	/** Hide the kiosk clock and date. */
	hideClockDate?: boolean;
	/** IANA timezone for the kiosk clock/date. */
	timezone?: string;
	/** Drop sensors and the [weather] block so the overlay can go fully empty. */
	minimalOverlay?: boolean;
};

export type PfServer = {
	baseURL: string;
	stop: () => Promise<void>;
};

const READY_TIMEOUT_MS = 15_000;
const READY_INTERVAL_MS = 150;

/** Spawns one dev-mode Go server (mocks) on its own port + temp dir. */
export async function startServer(opts: ServerOptions): Promise<PfServer> {
	// One retry covers the free-port → spawn race.
	let lastErr: unknown;
	for (let attempt = 0; attempt < 2; attempt++) {
		try {
			return await spawnOnce(opts);
		} catch (err) {
			lastErr = err;
		}
	}
	throw lastErr;
}

async function spawnOnce(opts: ServerOptions): Promise<PfServer> {
	const port = await freePort();
	const dir = await mkdtemp(path.join(os.tmpdir(), 'pf-e2e-run-'));
	const imagesDir = path.join(dir, 'images');
	await cp(opts.seedDir ?? seedImagesDir(), imagesDir, { recursive: true });
	await writeFile(
		path.join(dir, 'config.toml'),
		renderConfig({
			port,
			imagesDir,
			passwordHash: opts.passwordHash,
			immich: opts.immich,
			hideClockDate: opts.hideClockDate,
			timezone: opts.timezone,
			minimalOverlay: opts.minimalOverlay
		})
	);

	const env: NodeJS.ProcessEnv = { ...process.env, GO_ENV: 'dev', UPDATER_MOCK_DELAY: '0' };
	if (opts.wifiOff) env.WIFI_MOCK = 'off';
	if (opts.updateLatest) env.UPDATER_MOCK_LATEST = opts.updateLatest;
	if (opts.updateOutcome) env.UPDATER_MOCK_OUTCOME = opts.updateOutcome;
	if (opts.updateOffline) env.UPDATER_MOCK_OFFLINE = '1';

	const proc = spawn(
		binaryPath(),
		[
			'-config',
			'config.toml',
			'-overrides',
			'runtime-overrides.toml',
			'-screen-state',
			'screen-state'
		],
		{ cwd: dir, env }
	);

	const logs: string[] = [];
	proc.stdout.on('data', (d) => logs.push(String(d)));
	proc.stderr.on('data', (d) => logs.push(String(d)));

	const baseURL = `http://127.0.0.1:${port}`;
	const stop = async () => {
		proc.kill('SIGTERM');
		await new Promise<void>((resolve) => proc.once('exit', () => resolve()));
		await rm(dir, { recursive: true, force: true });
	};

	try {
		await waitForReady(baseURL, proc);
	} catch (err) {
		await stop();
		throw new Error(`${String(err)}\n--- server logs ---\n${logs.join('')}`, { cause: err });
	}

	return { baseURL, stop };
}

async function waitForReady(baseURL: string, proc: ChildProcess): Promise<void> {
	const deadline = Date.now() + READY_TIMEOUT_MS;
	while (Date.now() < deadline) {
		if (proc.exitCode !== null) {
			throw new Error(`server exited early with code ${proc.exitCode}`);
		}
		try {
			const res = await fetch(`${baseURL}/healthz`);
			if (res.ok) return;
		} catch {
			// not up yet
		}
		await new Promise((r) => setTimeout(r, READY_INTERVAL_MS));
	}
	throw new Error(`server not ready within ${READY_TIMEOUT_MS}ms`);
}
