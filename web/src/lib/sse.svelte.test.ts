import { tick } from 'svelte';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import type {
	ImagePayload,
	KioskPayload,
	ScreenPayload,
	SensorPayload,
	WeatherPayload
} from '$lib/api/types.gen';
import { SSESubscriber, IDLE_GRACE_MS } from './sse.svelte';

type SSEEvent =
	| { event: 'sensor'; data: SensorPayload }
	| { event: 'weather'; data: WeatherPayload }
	| { event: 'image'; data: ImagePayload }
	| { event: 'screen'; data: ScreenPayload }
	| { event: 'screen_aspect'; data: { aspect: number } }
	| { event: 'kiosk'; data: KioskPayload }
	| { event: 'ready'; data: Record<string, never> }
	| { event: 'ping'; data: Record<string, never> };

type Snapshot = {
	sensors: Record<string, SensorPayload>;
	weather: WeatherPayload | null;
	image: ImagePayload | null;
	screen: ScreenPayload | null;
	screenAspect: number | null;
	kiosk: KioskPayload | null;
	ready: boolean;
};

let emitEvent: (event: SSEEvent) => void;
// emitRaw pushes an unvalidated payload through the same onSseEvent path the
// real client uses, so tests can exercise the isSSEEvent guard with garbage.
let emitRaw: (raw: unknown) => void;
let endStream: () => void;
let connectCount = 0;

vi.mock('$lib/api/sdk.gen', () => ({
	apiEvents: (options?: { signal?: AbortSignal; onSseEvent?: (e: unknown) => void }) => {
		let resolve: (value: IteratorResult<unknown>) => void;
		let reject: (err: unknown) => void;
		connectCount++;

		// Mirror real fetch behaviour: abort the pending stream.next() call.
		options?.signal?.addEventListener('abort', () => {
			reject?.(new DOMException('Aborted', 'AbortError'));
		});

		const stream = {
			[Symbol.asyncIterator]() {
				return this;
			},
			next() {
				return new Promise<IteratorResult<unknown>>((res, rej) => {
					resolve = res;
					reject = rej;
				});
			},
			return() {
				return Promise.resolve({ done: true as const, value: undefined });
			}
		};

		emitEvent = (event: SSEEvent) => {
			options?.onSseEvent?.({ event: event.event, data: event.data });
			resolve({ done: false, value: undefined });
		};

		emitRaw = (raw: unknown) => {
			options?.onSseEvent?.(raw);
			resolve({ done: false, value: undefined });
		};

		endStream = () => {
			resolve({ done: true, value: undefined });
		};

		return Promise.resolve({ stream });
	}
}));

describe('SSESubscriber', () => {
	const cleanups: Array<() => void> = [];

	afterEach(() => {
		while (cleanups.length) cleanups.pop()!();
		connectCount = 0;
	});

	function watch(subscriber: SSESubscriber) {
		const snapshot = $state<Snapshot>({
			sensors: {},
			weather: null,
			image: null,
			screen: null,
			screenAspect: null,
			kiosk: null,
			ready: false
		});

		const stop = $effect.root(() => {
			$effect(() => {
				snapshot.sensors = subscriber.sensors;
				snapshot.weather = subscriber.weather;
				snapshot.image = subscriber.image;
				snapshot.screen = subscriber.screen;
				snapshot.screenAspect = subscriber.screenAspect;
				snapshot.kiosk = subscriber.kiosk;
				snapshot.ready = subscriber.ready;
			});
		});
		cleanups.push(stop);

		return { snapshot, stop };
	}

	async function watchFreshSubscriber() {
		const subscriber = new SSESubscriber();
		const watcher = watch(subscriber);
		await tick();
		return { subscriber, ...watcher };
	}

	describe('connection lifecycle', () => {
		it('connects when observed via reactive getters', async () => {
			const { snapshot } = await watchFreshSubscriber();
			expect(snapshot.ready).toBe(false);
		});

		it('keeps the stream warm when a reader returns within the idle grace', async () => {
			vi.useFakeTimers();
			try {
				const { subscriber, stop: stopFirst } = await watchFreshSubscriber();
				emitEvent({ event: 'image', data: { names: ['a.jpg'] } });
				await tick();
				expect(connectCount).toBe(1);

				stopFirst();
				await tick();

				// Re-subscribe before the grace elapses: no reconnect, state retained.
				vi.advanceTimersByTime(IDLE_GRACE_MS - 1);
				const second = watch(subscriber);
				await tick();

				expect(connectCount).toBe(1);
				expect(second.snapshot.image).toEqual({ names: ['a.jpg'] });
			} finally {
				vi.useRealTimers();
			}
		});

		it('clears state on disconnect and reconnect', async () => {
			vi.useFakeTimers();
			try {
				const { subscriber, stop: stopFirst } = await watchFreshSubscriber();

				emitEvent({ event: 'image', data: { names: ['a.jpg'] } });
				await tick();
				emitEvent({ event: 'ready', data: {} as Record<string, never> });
				await tick();

				stopFirst();
				await tick();

				// Let the grace window elapse so the stream actually disconnects.
				vi.advanceTimersByTime(IDLE_GRACE_MS);
				await tick();

				const second = watch(subscriber);
				await tick();

				expect(connectCount).toBe(2);
				expect(second.snapshot.image).toBeNull();
				expect(second.snapshot.ready).toBe(false);
				expect(second.snapshot.sensors).toEqual({});
			} finally {
				vi.useRealTimers();
			}
		});

		it('reconnects after stream ends and delivers new events', async () => {
			const { snapshot } = await watchFreshSubscriber();

			emitEvent({ event: 'ready', data: {} as Record<string, never> });
			await tick();
			expect(snapshot.ready).toBe(true);
			expect(connectCount).toBe(1);

			endStream();
			await tick();
			await tick();

			expect(snapshot.ready).toBe(false);
			expect(connectCount).toBe(2);

			emitEvent({ event: 'image', data: { names: ['reconnected.jpg'] } });
			await tick();
			expect(snapshot.image).toEqual({ names: ['reconnected.jpg'] });
		});
	});

	describe('sensor events', () => {
		it('indexes reading by role:kind when role is set', async () => {
			const { snapshot } = await watchFreshSubscriber();
			const payload: SensorPayload = {
				device_id: 'living_room',
				role: 'inside',
				kind: 'temperature',
				value: 23.45,
				timestamp: '2026-05-18T00:00:00Z'
			};

			emitEvent({ event: 'sensor', data: payload });
			await tick();

			expect(snapshot.sensors['inside:temperature']).toMatchObject({ value: 23.45 });
		});

		it('falls back to device_id:kind when role is absent', async () => {
			const { snapshot } = await watchFreshSubscriber();
			const payload: SensorPayload = {
				device_id: 'living_room',
				kind: 'humidity',
				value: 48.0,
				timestamp: '2026-05-18T00:00:00Z'
			};

			emitEvent({ event: 'sensor', data: payload });
			await tick();

			expect(snapshot.sensors['living_room:humidity']).toMatchObject({ value: 48.0 });
		});

		it('updates existing reading on subsequent events', async () => {
			const { snapshot } = await watchFreshSubscriber();
			const base: SensorPayload = {
				device_id: 'living_room',
				role: 'inside',
				kind: 'temperature',
				value: 20.0,
				timestamp: ''
			};

			emitEvent({ event: 'sensor', data: base });
			await tick();
			emitEvent({ event: 'sensor', data: { ...base, value: 21.5 } });
			await tick();

			expect(snapshot.sensors['inside:temperature'].value).toBe(21.5);
		});
	});

	describe('weather events', () => {
		it('updates weather state', async () => {
			const { snapshot } = await watchFreshSubscriber();

			emitEvent({ event: 'weather', data: { icon_code: '01d', temp: 18.0, humidity: 55 } });
			await tick();

			expect(snapshot.weather).toMatchObject({ icon_code: '01d', temp: 18.0 });
		});

		it('clears weather when icon_code is empty', async () => {
			const { snapshot } = await watchFreshSubscriber();

			emitEvent({ event: 'weather', data: { icon_code: '01d', temp: 18.0, humidity: 55 } });
			await tick();
			expect(snapshot.weather).not.toBeNull();

			emitEvent({ event: 'weather', data: { icon_code: '', temp: 0, humidity: 0 } });
			await tick();
			expect(snapshot.weather).toBeNull();
		});
	});

	describe('image events', () => {
		it('stores the payload when name is non-empty', async () => {
			const { snapshot } = await watchFreshSubscriber();

			emitEvent({ event: 'image', data: { names: ['a.jpg'] } });
			await tick();

			expect(snapshot.image).toEqual({ names: ['a.jpg'] });
		});

		it('clears the image when name is empty', async () => {
			const { snapshot } = await watchFreshSubscriber();

			emitEvent({ event: 'image', data: { names: ['a.jpg'] } });
			await tick();
			expect(snapshot.image).not.toBeNull();

			emitEvent({ event: 'image', data: { names: [] } });
			await tick();
			expect(snapshot.image).toBeNull();
		});
	});

	describe('screen events', () => {
		it('exposes live power and intent separately', async () => {
			const { snapshot } = await watchFreshSubscriber();
			expect(snapshot.screen).toBeNull();

			// Idle-blank: live power off, but auto-wake (intent) stays on.
			emitEvent({ event: 'screen', data: { on: false, auto: true } });
			await tick();
			expect(snapshot.screen).toEqual({ on: false, auto: true });

			// Motion wake: live power back on, intent unchanged.
			emitEvent({ event: 'screen', data: { on: true, auto: true } });
			await tick();
			expect(snapshot.screen).toEqual({ on: true, auto: true });
		});
	});

	describe('screen aspect events', () => {
		it('exposes the reported frame aspect ratio', async () => {
			const { snapshot } = await watchFreshSubscriber();
			expect(snapshot.screenAspect).toBeNull();

			emitEvent({ event: 'screen_aspect', data: { aspect: 1.7778 } });
			await tick();
			expect(snapshot.screenAspect).toBe(1.7778);
		});
	});

	describe('ready event', () => {
		it('flips ready to true', async () => {
			const { snapshot } = await watchFreshSubscriber();

			expect(snapshot.ready).toBe(false);
			emitEvent({ event: 'ready', data: {} as Record<string, never> });
			await tick();

			expect(snapshot.ready).toBe(true);
		});
	});

	describe('ping events', () => {
		it('does not modify subscriber state', async () => {
			const { snapshot } = await watchFreshSubscriber();

			emitEvent({ event: 'ping', data: {} as Record<string, never> });
			await tick();

			expect(snapshot.sensors).toEqual({});
			expect(snapshot.weather).toBeNull();
			expect(snapshot.image).toBeNull();
			expect(snapshot.ready).toBe(false);
		});
	});

	describe('kiosk events', () => {
		it('exposes the kiosk payload', async () => {
			const { snapshot } = await watchFreshSubscriber();
			expect(snapshot.kiosk).toBeNull();

			const kiosk: KioskPayload = {
				version: 'v1.0.0',
				labels: { outside: 'Out', inside: 'In', humidity: 'Hum' },
				locale: 'en-US',
				sensors: ['inside:temperature'],
				weather: true
			};
			emitEvent({ event: 'kiosk', data: kiosk });
			await tick();

			expect(snapshot.kiosk).toEqual(kiosk);
		});
	});

	describe('malformed events', () => {
		// The isSSEEvent guard must drop anything missing the {event, data} shape
		// before handleEvent runs, otherwise a bad frame corrupts or crashes state.
		it('ignores payloads rejected by the isSSEEvent guard', async () => {
			const { snapshot } = await watchFreshSubscriber();

			emitRaw(null);
			emitRaw('not-an-object');
			emitRaw({ event: 'sensor' }); // missing data
			emitRaw({ data: { device_id: 'x', kind: 'temperature', value: 1, timestamp: '' } }); // missing event
			await tick();

			expect(snapshot.sensors).toEqual({});
			expect(snapshot.weather).toBeNull();
			expect(snapshot.image).toBeNull();

			// A well-formed event after the garbage still lands, proving the stream
			// survived the rejected frames.
			emitEvent({
				event: 'sensor',
				data: { device_id: 'living_room', kind: 'humidity', value: 50, timestamp: '' }
			});
			await tick();
			expect(snapshot.sensors['living_room:humidity']).toMatchObject({ value: 50 });
		});
	});

	describe('watchdog', () => {
		beforeEach(() => vi.useFakeTimers());
		afterEach(() => vi.useRealTimers());

		it('reconnects when 75 s pass with no event', async () => {
			await watchFreshSubscriber();
			expect(connectCount).toBe(1);

			vi.advanceTimersByTime(75_000);
			await tick();
			await tick();

			expect(connectCount).toBe(2);
		});

		it('resets on any SSE event, delaying reconnect', async () => {
			await watchFreshSubscriber();
			expect(connectCount).toBe(1);

			// Advance to just before the deadline, then emit a ping.
			vi.advanceTimersByTime(74_000);
			emitEvent({ event: 'ping', data: {} as Record<string, never> });
			await tick();

			// 74 s after the ping is still within the new 75 s window.
			vi.advanceTimersByTime(74_000);
			await tick();
			expect(connectCount).toBe(1);

			// Cross the threshold: watchdog fires and reconnects.
			vi.advanceTimersByTime(1_001);
			await tick();
			await tick();
			expect(connectCount).toBe(2);
		});
	});
});
