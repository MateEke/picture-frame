import { describe, it, expect } from 'vitest';
import { arrayMove, moveUp, moveDown, moveToStart, moveToEnd } from './reorder';

describe('reorder helpers', () => {
	const base = ['a', 'b', 'c', 'd'];

	it('arrayMove moves an item and is non-mutating', () => {
		const out = arrayMove(base, 0, 2);
		expect(out).toEqual(['b', 'c', 'a', 'd']);
		expect(base).toEqual(['a', 'b', 'c', 'd']);
	});

	it('moveUp swaps with previous, no-op at top', () => {
		expect(moveUp(base, 2)).toEqual(['a', 'c', 'b', 'd']);
		expect(moveUp(base, 0)).toEqual(base);
	});

	it('moveDown swaps with next, no-op at bottom', () => {
		expect(moveDown(base, 1)).toEqual(['a', 'c', 'b', 'd']);
		expect(moveDown(base, 0)).toEqual(['b', 'a', 'c', 'd']);
		expect(moveDown(base, 3)).toEqual(base);
	});

	it('moveToStart moves to front, no-op when already first', () => {
		expect(moveToStart(base, 2)).toEqual(['c', 'a', 'b', 'd']);
		expect(moveToStart(base, 0)).toEqual(base);
	});

	it('moveToEnd moves to back, no-op when already last', () => {
		expect(moveToEnd(base, 1)).toEqual(['a', 'c', 'd', 'b']);
		expect(moveToEnd(base, 0)).toEqual(['b', 'c', 'd', 'a']);
		expect(moveToEnd(base, 3)).toEqual(base);
	});
});
