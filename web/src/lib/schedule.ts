import type { NextChangeDto } from '#lib/api/types.gen.js';

export function formatClock(hhmm: string, locale: string): string {
	const [h, m] = hhmm.split(':').map(Number);
	const at = new Date(Date.UTC(2000, 0, 1, h, m));
	const opts: Intl.DateTimeFormatOptions = { timeStyle: 'short', timeZone: 'UTC' };
	try {
		return at.toLocaleTimeString(locale || undefined, opts);
	} catch {
		// display.locale is free text in config.toml; an invalid tag throws.
		return at.toLocaleTimeString(undefined, opts);
	}
}

export function scheduleError(off: string, on: string): string | null {
	return off && off === on ? "Off and on times can't be the same." : null;
}

export function scheduleSummary(off: string, on: string, locale: string): string {
	const offAt = off && formatClock(off, locale);
	const onAt = on && formatClock(on, locale);
	if (offAt && onAt) {
		return off > on
			? `Off every night from ${offAt} to ${onAt}.`
			: `Off every day from ${offAt} to ${onAt}.`;
	}
	if (offAt) return `Turns off every day at ${offAt} and stays off until you turn it back on.`;
	if (onAt) return `Turns on every day at ${onAt}.`;
	return 'No schedule. Set a time to turn the screen off or on every day.';
}

// `at` carries the frame's UTC offset, so "tomorrow" holds in any browser time zone.
export function nextChangeTime(next: NextChangeDto, locale: string, now: number): string {
	const clock = formatClock(next.time, locale);
	return frameDate(next.at, now) === next.at.slice(0, 10) ? clock : `${clock} tomorrow`;
}

function frameDate(at: string, now: number): string {
	const m = /([+-])(\d{2}):(\d{2})$/.exec(at);
	const offsetMin = m ? (m[1] === '-' ? -1 : 1) * (Number(m[2]) * 60 + Number(m[3])) : 0;
	return new Date(now + offsetMin * 60_000).toISOString().slice(0, 10);
}
