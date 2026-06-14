import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Fader } from './fader.svelte';

describe('Fader', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('starts with empty sources and 0.1% opacity', () => {
		const f = new Fader();
		expect(f.bottomSrc).toBe('');
		expect(f.topSrc).toBe('');
		expect(f.topOp).toBe(0.001);
		expect(f.transitioning).toBe(false);
	});

	it('initializes the first image directly into bottomSrc without fading', () => {
		const f = new Fader();
		f.show('/img/1');

		expect(f.bottomSrc).toBe('/img/1');
		expect(f.topSrc).toBe('');
		expect(f.transitioning).toBe(false);

		// Prove it didn't lock the fader (loading is false)
		f.show('/img/2');
		expect(f.topSrc).toBe('/img/2');
	});

	it('starts fade-in after top image loads and delay passes', () => {
		const f = new Fader();
		f.show('/img/1'); // Initial load
		f.show('/img/2'); // Starts crossfade

		expect(f.topSrc).toBe('/img/2');
		expect(f.transitioning).toBe(true);
		expect(f.topOp).toBe(0.001); // Hasn't faded yet

		f.onTopLoad();

		vi.advanceTimersByTime(999);
		expect(f.topOp).toBe(0.001);

		vi.advanceTimersByTime(1);
		expect(f.topOp).toBe(1); // Fade in executes!
	});

	it('swaps bottom source and snaps top layer back after transition completes', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2');
		f.onTopLoad();
		vi.advanceTimersByTime(1000); // Fade in finishes

		expect(f.topOp).toBe(1);

		// 1. Simulate CSS transition ending
		f.onTransitionEnd();
		expect(f.bottomSrc).toBe('/img/2'); // The secret swap

		// 2. Simulate the bottom layer finishing its background decode
		f.onBottomLoad();
		expect(f.transitioning).toBe(false);
		expect(f.topOp).toBe(0.001); // The invisible snap back
	});

	it('drops concurrent show() requests while a fade is in flight', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2'); // Locks the fader

		f.show('/img/3'); // Should be ignored
		expect(f.topSrc).toBe('/img/2');
	});

	it('no-ops when the URL is already showing in either layer', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/1'); // Already in bottomSrc
		expect(f.transitioning).toBe(false);
		expect(f.topSrc).toBe('');
	});

	it('successfully completes multiple full crossfade cycles', () => {
		const f = new Fader();

		// Cycle 1: Initialization
		f.show('/img/a');

		// Cycle 2: Crossfade to 'b'
		f.show('/img/b');
		f.onTopLoad();
		vi.advanceTimersByTime(1000);
		f.onTransitionEnd();
		f.onBottomLoad();

		// Cycle 3: Crossfade to 'c'
		f.show('/img/c');
		expect(f.topSrc).toBe('/img/c');
		expect(f.transitioning).toBe(true);

		f.onTopLoad();
		vi.advanceTimersByTime(1000);
		expect(f.topOp).toBe(1);

		f.onTransitionEnd();
		expect(f.bottomSrc).toBe('/img/c');

		f.onBottomLoad();
		expect(f.transitioning).toBe(false);
		expect(f.topOp).toBe(0.001);
	});

	it('ignores stray onLoad events if not actively transitioning', () => {
		const f = new Fader();
		f.show('/img/1');

		// These should do absolutely nothing because `loading` is false
		f.onTopLoad();
		vi.advanceTimersByTime(1000);
		expect(f.topOp).toBe(0.001);

		f.onBottomLoad();
		expect(f.topOp).toBe(0.001);
	});

	it('honours custom switch-delay timings', () => {
		const f = new Fader(500); // 500ms instead of 1000ms
		f.show('/img/1');
		f.show('/img/2');
		f.onTopLoad();

		vi.advanceTimersByTime(499);
		expect(f.topOp).toBe(0.001);

		vi.advanceTimersByTime(1);
		expect(f.topOp).toBe(1);
	});

	it('stop() forcefully drops the loading lock', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2'); // Locks the fader

		f.stop(); // Manually break the lock

		f.show('/img/3'); // Should now be accepted
		expect(f.topSrc).toBe('/img/3');
	});

	it('does not promote the bottom layer on a transition end before the fade completes', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2'); // topOp still 0.001, fade has not run

		f.onTransitionEnd(); // stray/early transitionend; topOp !== 1

		expect(f.bottomSrc).toBe('/img/1'); // must NOT promote /img/2 yet
	});

	it('ignores a bottom-layer load while the crossfade is still mid-flight', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2'); // loading, bottomSrc '/img/1' !== topSrc '/img/2'

		f.onBottomLoad(); // bottomSrc !== topSrc → must not reset the transition

		expect(f.transitioning).toBe(true);
		expect(f.topSrc).toBe('/img/2');
	});

	it('recovers from a failed top-image load and accepts the next show()', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2'); // locks the fader
		f.onTopError(); // /img/2 404s (e.g. deleted between SSE event and fetch)

		expect(f.transitioning).toBe(false);
		expect(f.topSrc).toBe('/img/1'); // snapped back to the visible image
		expect(f.bottomSrc).toBe('/img/1');

		f.show('/img/3'); // must not be locked out
		expect(f.topSrc).toBe('/img/3');
		expect(f.transitioning).toBe(true);
	});

	it('ignores a stray top error when no crossfade is in flight', () => {
		const f = new Fader();
		f.show('/img/1');
		f.onTopError();
		expect(f.bottomSrc).toBe('/img/1');
		expect(f.transitioning).toBe(false);
	});

	it('self-resets a stalled crossfade after the stall window', () => {
		const f = new Fader(1000, 30_000);
		f.show('/img/1');
		f.show('/img/2'); // top never loads and never errors (hung connection)

		vi.advanceTimersByTime(29_999);
		expect(f.transitioning).toBe(true);

		vi.advanceTimersByTime(1);
		expect(f.transitioning).toBe(false);

		f.show('/img/3'); // recovered: next image goes through
		expect(f.topSrc).toBe('/img/3');
	});

	it('does not let the stall timer fire after a completed crossfade', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2');
		f.onTopLoad();
		vi.advanceTimersByTime(1000);
		f.onTransitionEnd();
		f.onBottomLoad(); // cycle complete; stall timer must be disarmed

		vi.advanceTimersByTime(60_000);
		expect(f.bottomSrc).toBe('/img/2');
		expect(f.topSrc).toBe('/img/2');
	});

	it('suppresses a pending fade-in that outlives an abandoned transition', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2');
		f.onTopLoad(); // arms the delayed fade-in
		f.onTopError(); // abandoned before the delay elapses

		vi.advanceTimersByTime(1000);
		expect(f.topOp).toBe(0.001); // the stale fade-in must not fire
	});

	it('clears a failed first image so a later show() can retry it', () => {
		const f = new Fader();
		f.show('/img/1');
		f.onBottomError(); // first image 404s

		expect(f.bottomSrc).toBe('');
		f.show('/img/1'); // same URL again, must not be deduped away
		expect(f.bottomSrc).toBe('/img/1');
	});

	it('treats a bottom error during promotion like a top error', () => {
		const f = new Fader();
		f.show('/img/1');
		f.show('/img/2');
		f.onTopLoad();
		vi.advanceTimersByTime(1000);
		f.onTransitionEnd(); // bottomSrc promoted to /img/2
		f.onBottomError(); // the promoted bottom fails to load

		expect(f.transitioning).toBe(false);
		f.show('/img/3'); // not locked
		expect(f.topSrc).toBe('/img/3');
	});

	it('ignores a bottom error when nothing has been shown yet', () => {
		const f = new Fader();
		f.onBottomError(); // empty-src error event at startup
		expect(f.bottomSrc).toBe('');

		f.show('/img/1');
		expect(f.bottomSrc).toBe('/img/1');
	});

	it('stop() disarms the stall timer', () => {
		const f = new Fader(1000, 30_000);
		f.show('/img/1');
		f.show('/img/2');
		f.stop();

		vi.advanceTimersByTime(60_000);
		// No stall reset ran against the stopped fader: top layer untouched.
		expect(f.topSrc).toBe('/img/2');
	});
});
