import { createContext } from 'svelte';
import { createSubscriber } from 'svelte/reactivity';
import { sensorKey } from './helpers';
import { apiEvents } from '$lib/api/sdk.gen';
import type {
	EventsResponse,
	SensorPayload,
	WeatherPayload,
	ImagePayload,
	ScreenPayload,
	KioskPayload
} from '$lib/api/types.gen';

type SSEEvent = NonNullable<EventsResponse>[number];

function isSSEEvent(v: unknown): v is SSEEvent {
	return v != null && typeof v === 'object' && 'event' in v && 'data' in v;
}

// 30 s server ping interval + 45 s grace before the stream is considered dead.
const WATCHDOG_MS = 75_000;

// Stay connected this long after the last reader leaves, so fast tab switching
// doesn't churn the stream (the reconnect can wedge Svelte's scheduler).
export const IDLE_GRACE_MS = 5_000;

export class SSESubscriber {
	private subscribe: ReturnType<typeof createSubscriber>;
	private controller: AbortController | null = null;
	private watchdog: ReturnType<typeof setTimeout> | null = null;
	private idleTimer: ReturnType<typeof setTimeout> | null = null;

	private _ready = $state(false);
	private _image = $state<ImagePayload | null>(null);
	private _sensors = $state<Record<string, SensorPayload>>({});
	private _weather = $state<WeatherPayload | null>(null);
	private _screen = $state<ScreenPayload | null>(null);
	private _screenAspect = $state<number | null>(null);
	private _kiosk = $state<KioskPayload | null>(null);

	constructor() {
		this.subscribe = createSubscriber(() => {
			if (this.idleTimer !== null) {
				clearTimeout(this.idleTimer);
				this.idleTimer = null;
			}
			this.connect();
			return () => {
				this.idleTimer = setTimeout(() => {
					this.idleTimer = null;
					this.disconnect();
				}, IDLE_GRACE_MS);
			};
		});
	}

	private async connect() {
		if (this.controller) return;

		this._image = null;
		this._ready = false;
		this._sensors = {};
		this._weather = null;
		this._screen = null;
		this._screenAspect = null;
		this._kiosk = null;

		const controller = new AbortController();
		this.controller = controller;

		while (!controller.signal.aborted) {
			const streamCtl = new AbortController();
			this.resetWatchdog(() => streamCtl.abort());

			try {
				const { stream } = await apiEvents({
					signal: AbortSignal.any([controller.signal, streamCtl.signal]),
					onSseEvent: (raw) => {
						if (isSSEEvent(raw)) {
							this.resetWatchdog(() => streamCtl.abort());
							this.handleEvent(raw);
						}
					}
				});
				let result = await stream.next();
				while (!result.done) {
					result = await stream.next();
				}
			} catch {
				// stream ended or aborted
			} finally {
				this.clearWatchdog();
			}

			if (controller.signal.aborted) break;
			this._ready = false;
		}
	}

	private resetWatchdog(abort: () => void): void {
		if (this.watchdog !== null) clearTimeout(this.watchdog);
		this.watchdog = setTimeout(abort, WATCHDOG_MS);
	}

	private clearWatchdog(): void {
		if (this.watchdog !== null) {
			clearTimeout(this.watchdog);
			this.watchdog = null;
		}
	}

	private handleEvent(event: SSEEvent): void {
		switch (event.event) {
			case 'sensor':
				this._sensors[sensorKey(event.data)] = event.data;
				break;
			case 'weather':
				this._weather = event.data.icon_code ? event.data : null;
				break;
			case 'image':
				this._image = event.data.names?.length ? event.data : null;
				break;
			case 'screen':
				this._screen = event.data;
				break;
			case 'screen_aspect':
				this._screenAspect = event.data.aspect || null;
				break;
			case 'kiosk':
				this._kiosk = event.data;
				break;
			case 'ready':
				this._ready = true;
				break;
		}
	}

	private disconnect(): void {
		this.clearWatchdog();
		this.controller?.abort();
		this.controller = null;
		this._ready = false;
	}

	get sensors() {
		this.subscribe();
		return this._sensors;
	}
	get weather() {
		this.subscribe();
		return this._weather;
	}
	get image() {
		this.subscribe();
		return this._image;
	}
	get screen() {
		this.subscribe();
		return this._screen;
	}
	get screenAspect() {
		this.subscribe();
		return this._screenAspect;
	}
	get kiosk() {
		this.subscribe();
		return this._kiosk;
	}
	get ready() {
		this.subscribe();
		return this._ready;
	}
}

export const [getSSEContext, setSSEContext] = createContext<SSESubscriber>();
