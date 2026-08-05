// The overlay is the only static high-contrast content on screen, so it orbits a
// small ellipse to spare OLED panels.

/** Horizontal amplitude, in rem. One glyph stem; below that only edges smear, not peak wear. */
export const SHIFT_X_REM = 0.75;
/** Vertical amplitude, in rem. Less room below the overlay than beside it. */
export const SHIFT_Y_REM = 0.5;

/** Offset for the given time, in rem. One orbit per hour. */
export function overlayShift(date: Date): { x: number; y: number } {
	const angle = (date.getMinutes() / 60) * 2 * Math.PI;
	return { x: SHIFT_X_REM * Math.cos(angle), y: SHIFT_Y_REM * Math.sin(angle) };
}

// Whole pixels: the slides underneath are GPU-promoted, and a fractional translate on
// a composited layer resamples and softens the text.
function toStyle(x: number, y: number, rootFontSizePx: number): string {
	const px = (rem: number): number => Math.round(rem * rootFontSizePx);
	return `translate(${px(x)}px, ${px(y)}px)`;
}

/**
 * Styles for the clock (`lead`) and readings (`trail`). Landscape mirrors `trail`
 * horizontally so both screen margins move together; portrait stacks them left-aligned,
 * where mirroring would pull them apart.
 */
export function overlayShiftPair(
	date: Date,
	rootFontSizePx: number,
	portrait: boolean
): { lead: string; trail: string } {
	const { x, y } = overlayShift(date);
	return {
		lead: toStyle(x, y, rootFontSizePx),
		trail: toStyle(portrait ? x : -x, y, rootFontSizePx)
	};
}
