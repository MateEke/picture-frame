// Logic behind the on-device touch UI (/kiosk): API wrappers plus the pure
// helpers its views share. Kept out of the components so it's unit-testable.
import {
	apiDeleteFile,
	apiDeleteImage,
	apiGetSystemInfo,
	apiGetTouchSettings,
	apiGetWifiStatus,
	apiListFiles,
	apiListImages,
	apiPutTouchSettings,
	apiScreenWake,
	apiSetScreen,
	apiSetSlideshowSelection
} from '$lib/api/sdk.gen';
import type { FileItem, ImageItem, TouchSettingsBody, TouchSettingsDto } from '$lib/api/types.gen';

export type { FileItem, ImageItem, TouchSettingsBody, TouchSettingsDto };

// --- API ---

export async function fetchImages(): Promise<ImageItem[] | null> {
	const { data, error } = await apiListImages();
	return error ? null : (data ?? []);
}

export async function setIncluded(names: string[], included: boolean): Promise<boolean> {
	if (names.length === 0) return true;
	const { error } = await apiSetSlideshowSelection({ body: { names, included } });
	return !error;
}

/** Deletes one at a time (the server has no batch route); returns how many failed. */
export async function removeImages(names: string[]): Promise<number> {
	let failed = 0;
	for (const name of names) {
		const { error } = await apiDeleteImage({ path: { name } });
		if (error) failed++;
	}
	return failed;
}

export interface FileListing {
	files: FileItem[];
	freeBytes: number | null;
}

export async function fetchFiles(): Promise<FileListing | null> {
	const { data, error } = await apiListFiles();
	if (error || !data) return null;
	return { files: data.files ?? [], freeBytes: data.free_bytes ?? null };
}

export async function removeFile(name: string): Promise<boolean> {
	const { error } = await apiDeleteFile({ path: { name } });
	return !error;
}

export async function fetchSettings(): Promise<TouchSettingsBody | null> {
	const { data, error } = await apiGetTouchSettings();
	return error ? null : (data ?? null);
}

export async function saveSettings(s: TouchSettingsDto): Promise<string | null> {
	const { error } = await apiPutTouchSettings({ body: s });
	if (!error) return null;
	return (error as { detail?: string }).detail ?? 'error';
}

export interface DeviceInfo {
	ip: string;
	hostname: string;
	version: string;
	ssid: string;
}

export async function fetchDeviceInfo(): Promise<DeviceInfo> {
	const [sys, wifi] = await Promise.allSettled([apiGetSystemInfo(), apiGetWifiStatus()]);
	const s = sys.status === 'fulfilled' ? sys.value.data : undefined;
	const w = wifi.status === 'fulfilled' ? wifi.value.data : undefined;
	return {
		ip: s?.ip || w?.ip || '',
		hostname: s?.hostname || w?.hostname || '',
		version: s?.version ?? '',
		ssid: w?.ssid ?? ''
	};
}

export function screenOff(): void {
	apiSetScreen({ body: { state: 'off' } }).catch(() => undefined);
}

export function wake(): void {
	apiScreenWake().catch(() => undefined);
}

// --- Pure helpers ---

/** The address a phone on the same network opens to send photos. */
export function uploadUrl(info: Pick<DeviceInfo, 'ip' | 'hostname'>, port: string): string {
	const host = info.ip || (info.hostname ? `${info.hostname}.local` : '');
	if (!host) return '';
	const suffix = port && port !== '80' ? `:${port}` : '';
	return `http://${host}${suffix}/admin/images`;
}

export type FileKind = 'image' | 'video' | 'audio' | 'pdf' | 'text' | 'other';

export function fileKind(mime: string): FileKind {
	const m = mime.toLowerCase();
	if (m.startsWith('image/')) return 'image';
	if (m.startsWith('video/')) return 'video';
	if (m.startsWith('audio/')) return 'audio';
	if (m === 'application/pdf') return 'pdf';
	if (
		m.startsWith('text/') ||
		m.startsWith('application/json') ||
		m.startsWith('application/xml') ||
		m.startsWith('application/javascript')
	)
		return 'text';
	return 'other';
}

const numberFi = new Intl.NumberFormat('fi-FI', { maximumFractionDigits: 1 });

const unitsFi = ['t', 'kt', 'Mt', 'Gt', 'Tt'];
export const unitsEn = ['B', 'kB', 'MB', 'GB', 'TB'];

/** Byte sizes, Finnish by default: "850 t", "1,2 Mt", "3,4 Gt". */
export function formatBytes(n: number, units: readonly string[] = unitsFi): string {
	let v = Math.max(0, n);
	let i = 0;
	while (v >= 1000 && i < units.length - 1) {
		v /= 1000;
		i++;
	}
	return `${i === 0 ? Math.round(v) : numberFi.format(v)} ${units[i]}`;
}

/** "30 s", "2 min", "1 h 30 min"; 0 reads as "never" for idle delays. */
export function formatSeconds(sec: number, zero = 'Ei koskaan'): string {
	if (sec <= 0) return zero;
	if (sec < 60) return `${sec} s`;
	const h = Math.floor(sec / 3600);
	const m = Math.round((sec % 3600) / 60);
	if (h === 0) return `${m} min`;
	return m ? `${h} h ${m} min` : `${h} h`;
}

/** Parses a Go duration string ("2m0s", "30s", "1h30m") to whole seconds. */
export function parseGoDuration(d: string): number {
	if (!d) return 0;
	let total = 0;
	const re = /(\d+(?:\.\d+)?)(h|ms|m|s)/g;
	let match: RegExpExecArray | null;
	while ((match = re.exec(d)) !== null) {
		const v = parseFloat(match[1]);
		switch (match[2]) {
			case 'h':
				total += v * 3600;
				break;
			case 'm':
				total += v * 60;
				break;
			case 's':
				total += v;
				break;
			case 'ms':
				total += v / 1000;
				break;
		}
	}
	return Math.round(total);
}

export function toGoDuration(sec: number): string {
	return `${Math.max(0, Math.round(sec))}s`;
}

/** Steps a choice list: the next (dir=1) or previous (-1) option from current. */
export function stepChoice(options: readonly number[], current: number, dir: 1 | -1): number {
	const idx = options.findIndex((o) => o >= current);
	if (idx === -1) return dir === 1 ? options[options.length - 1] : options[options.length - 1];
	if (options[idx] !== current) {
		// Between two options: snap to the neighbour in the direction of travel.
		return dir === 1 ? options[idx] : options[Math.max(0, idx - 1)];
	}
	return options[Math.min(options.length - 1, Math.max(0, idx + dir))];
}

/** Shifts "HH:MM" by minutes, wrapping around midnight. */
export function shiftClock(hhmm: string, minutes: number): string {
	const [h, m] = hhmm.split(':').map((x) => parseInt(x, 10));
	const base = (Number.isFinite(h) ? h : 0) * 60 + (Number.isFinite(m) ? m : 0);
	const t = (((base + minutes) % 1440) + 1440) % 1440;
	return `${String(Math.floor(t / 60)).padStart(2, '0')}:${String(t % 60).padStart(2, '0')}`;
}

export interface GridWindow {
	firstRow: number;
	lastRow: number; // exclusive
}

/** Rows of a virtual grid to render for the current scroll position. */
export function visibleRows(
	scrollTop: number,
	viewport: number,
	rowHeight: number,
	rowCount: number,
	overscan = 2
): GridWindow {
	if (rowHeight <= 0 || rowCount <= 0) return { firstRow: 0, lastRow: 0 };
	const first = Math.max(0, Math.floor(scrollTop / rowHeight) - overscan);
	const last = Math.min(rowCount, Math.ceil((scrollTop + viewport) / rowHeight) + overscan);
	return { firstRow: first, lastRow: Math.max(first, last) };
}

export type Gesture = 'tap' | 'next' | 'prev' | null;

/**
 * Classifies a pointer gesture: a short, still press is a tap; a mostly
 * horizontal move past the threshold is a swipe (left = next, like paging).
 */
export function classifyGesture(dx: number, dy: number, ms: number, threshold = 60): Gesture {
	const ax = Math.abs(dx);
	const ay = Math.abs(dy);
	if (ax < 12 && ay < 12 && ms < 600) return 'tap';
	if (ax >= threshold && ax > ay * 1.5) return dx < 0 ? 'next' : 'prev';
	return null;
}

/**
 * Calls onIdle once after ms without a poke. ms <= 0 disables it. Used to hand
 * the screen back to the slideshow when nobody touches the menu.
 */
export class IdleTimer {
	private timer: ReturnType<typeof setTimeout> | null = null;

	constructor(
		private ms: number,
		private readonly onIdle: () => void
	) {}

	poke(): void {
		this.stop();
		if (this.ms > 0) this.timer = setTimeout(this.onIdle, this.ms);
	}

	setDelay(ms: number): void {
		this.ms = ms;
		if (this.timer !== null) this.poke();
	}

	stop(): void {
		if (this.timer !== null) {
			clearTimeout(this.timer);
			this.timer = null;
		}
	}
}

/**
 * Runs fn at most once per interval (leading edge). Touch wake-ups go through
 * this so a finger dragging around the gallery isn't a request per event.
 */
export function throttle(fn: () => void, intervalMs: number, now = () => Date.now()): () => void {
	let last = -Infinity;
	return () => {
		const t = now();
		if (t - last >= intervalMs) {
			last = t;
			fn();
		}
	};
}
