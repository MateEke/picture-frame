import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	IdleTimer,
	classifyGesture,
	fileKind,
	formatBytes,
	formatSeconds,
	parseGoDuration,
	shiftClock,
	stepChoice,
	throttle,
	toGoDuration,
	uploadUrl,
	visibleRows
} from './touch';

describe('uploadUrl', () => {
	it('uses the IP and hides port 80', () => {
		expect(uploadUrl({ ip: '192.168.1.20', hostname: 'frame' }, '80')).toBe(
			'http://192.168.1.20/admin/images'
		);
		expect(uploadUrl({ ip: '192.168.1.20', hostname: 'frame' }, '')).toBe(
			'http://192.168.1.20/admin/images'
		);
	});
	it('keeps a non-default port', () => {
		expect(uploadUrl({ ip: '10.0.0.2', hostname: '' }, '8080')).toBe(
			'http://10.0.0.2:8080/admin/images'
		);
	});
	it('falls back to mDNS, then to nothing', () => {
		expect(uploadUrl({ ip: '', hostname: 'frame' }, '80')).toBe('http://frame.local/admin/images');
		expect(uploadUrl({ ip: '', hostname: '' }, '80')).toBe('');
	});
});

describe('fileKind', () => {
	it.each([
		['image/png', 'image'],
		['video/mp4', 'video'],
		['audio/mpeg', 'audio'],
		['application/pdf', 'pdf'],
		['text/plain; charset=utf-8', 'text'],
		['application/json', 'text'],
		['application/zip', 'other'],
		['VIDEO/QuickTime', 'video']
	])('%s → %s', (mime, kind) => {
		expect(fileKind(mime)).toBe(kind);
	});
});

describe('formatBytes', () => {
	it('uses Finnish units and decimal comma', () => {
		expect(formatBytes(850)).toBe('850 t');
		expect(formatBytes(1_200_000)).toBe('1,2 Mt');
		expect(formatBytes(3_400_000_000)).toBe('3,4 Gt');
		expect(formatBytes(-5)).toBe('0 t');
		expect(formatBytes(2_500_000, ['B', 'kB', 'MB'])).toBe('2,5 MB');
	});
});

describe('formatSeconds', () => {
	it('formats durations', () => {
		expect(formatSeconds(0)).toBe('Ei koskaan');
		expect(formatSeconds(0, 'Pois')).toBe('Pois');
		expect(formatSeconds(30)).toBe('30 s');
		expect(formatSeconds(120)).toBe('2 min');
		expect(formatSeconds(3600)).toBe('1 h');
		expect(formatSeconds(5400)).toBe('1 h 30 min');
	});
});

describe('Go durations', () => {
	it('parses what the server sends', () => {
		expect(parseGoDuration('2m0s')).toBe(120);
		expect(parseGoDuration('1h30m0s')).toBe(5400);
		expect(parseGoDuration('30s')).toBe(30);
		expect(parseGoDuration('500ms')).toBe(1);
		expect(parseGoDuration('')).toBe(0);
		expect(parseGoDuration('0s')).toBe(0);
	});
	it('formats seconds back', () => {
		expect(toGoDuration(90)).toBe('90s');
		expect(toGoDuration(-3)).toBe('0s');
	});
});

describe('stepChoice', () => {
	const opts = [10, 30, 60, 120] as const;
	it('steps between options and clamps at the ends', () => {
		expect(stepChoice(opts, 30, 1)).toBe(60);
		expect(stepChoice(opts, 30, -1)).toBe(10);
		expect(stepChoice(opts, 120, 1)).toBe(120);
		expect(stepChoice(opts, 10, -1)).toBe(10);
	});
	it('snaps an off-list value to the neighbour in the direction of travel', () => {
		expect(stepChoice(opts, 45, 1)).toBe(60);
		expect(stepChoice(opts, 45, -1)).toBe(30);
		expect(stepChoice(opts, 500, -1)).toBe(120);
	});
});

describe('shiftClock', () => {
	it('wraps around midnight', () => {
		expect(shiftClock('23:00', 15)).toBe('23:15');
		expect(shiftClock('23:45', 30)).toBe('00:15');
		expect(shiftClock('00:10', -15)).toBe('23:55');
		expect(shiftClock('bad', 60)).toBe('01:00');
	});
});

describe('visibleRows', () => {
	it('renders the rows in view plus overscan', () => {
		expect(visibleRows(0, 1000, 250, 100)).toEqual({ firstRow: 0, lastRow: 6 });
		expect(visibleRows(2500, 1000, 250, 100)).toEqual({ firstRow: 8, lastRow: 16 });
	});
	it('clamps to the row count and handles empty grids', () => {
		expect(visibleRows(0, 1000, 250, 3)).toEqual({ firstRow: 0, lastRow: 3 });
		expect(visibleRows(0, 1000, 0, 3)).toEqual({ firstRow: 0, lastRow: 0 });
		expect(visibleRows(0, 1000, 250, 0)).toEqual({ firstRow: 0, lastRow: 0 });
	});
});

describe('classifyGesture', () => {
	it('tells taps from swipes', () => {
		expect(classifyGesture(2, 3, 120)).toBe('tap');
		expect(classifyGesture(2, 3, 900)).toBeNull(); // long press
		expect(classifyGesture(-120, 10, 200)).toBe('next');
		expect(classifyGesture(120, -10, 200)).toBe('prev');
		expect(classifyGesture(80, 90, 200)).toBeNull(); // diagonal
		expect(classifyGesture(30, 0, 200)).toBeNull(); // too short
	});
});

describe('IdleTimer', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('fires once after the delay and resets on poke', () => {
		const onIdle = vi.fn();
		const t = new IdleTimer(1000, onIdle);
		t.poke();
		vi.advanceTimersByTime(900);
		t.poke();
		vi.advanceTimersByTime(900);
		expect(onIdle).not.toHaveBeenCalled();
		vi.advanceTimersByTime(200);
		expect(onIdle).toHaveBeenCalledTimes(1);
	});

	it('is disabled by a zero delay and can be re-armed', () => {
		const onIdle = vi.fn();
		const t = new IdleTimer(0, onIdle);
		t.poke();
		vi.advanceTimersByTime(10_000);
		expect(onIdle).not.toHaveBeenCalled();
		t.setDelay(500);
		t.poke();
		vi.advanceTimersByTime(500);
		expect(onIdle).toHaveBeenCalledTimes(1);
	});

	it('setDelay restarts a running countdown and stop cancels it', () => {
		const onIdle = vi.fn();
		const t = new IdleTimer(1000, onIdle);
		t.poke();
		t.setDelay(3000);
		vi.advanceTimersByTime(2000);
		expect(onIdle).not.toHaveBeenCalled();
		t.stop();
		vi.advanceTimersByTime(5000);
		expect(onIdle).not.toHaveBeenCalled();
	});
});

describe('throttle', () => {
	it('runs on the leading edge at most once per interval', () => {
		let now = 0;
		const fn = vi.fn();
		const t = throttle(fn, 1000, () => now);
		t();
		t();
		expect(fn).toHaveBeenCalledTimes(1);
		now = 999;
		t();
		expect(fn).toHaveBeenCalledTimes(1);
		now = 1000;
		t();
		expect(fn).toHaveBeenCalledTimes(2);
	});
});
