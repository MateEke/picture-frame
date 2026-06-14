import { describe, it, expect } from 'vitest';
import { version } from '$app/environment';
import { shouldReload } from './versionReload.svelte';

describe('shouldReload', () => {
	const now = 1_700_000_000_000;

	it('does not reload when the backend has not reported a version yet', () => {
		expect(shouldReload(undefined, 0, now)).toBe(false);
		expect(shouldReload('', 0, now)).toBe(false);
	});

	it('does not reload when the backend matches this bundle', () => {
		expect(shouldReload(version, 0, now)).toBe(false);
	});

	it('reloads on a version mismatch', () => {
		expect(shouldReload('v9.9.9', 0, now)).toBe(true);
	});

	it('throttles a reload within the window and allows it at the boundary', () => {
		expect(shouldReload('v9.9.9', now - 59_999, now)).toBe(false);
		expect(shouldReload('v9.9.9', now - 60_000, now)).toBe(true);
	});
});
