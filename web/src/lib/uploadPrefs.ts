import { browser } from '$app/environment';

export interface CropRatio {
	id: string;
	w: number;
	h: number;
}

// Display order; first is the default.
export const CROP_RATIOS: CropRatio[] = [
	{ id: '16:9', w: 16, h: 9 },
	{ id: '9:16', w: 9, h: 16 },
	{ id: '4:3', w: 4, h: 3 },
	{ id: '1:1', w: 1, h: 1 }
];

const DEFAULT_RATIO = CROP_RATIOS[0];
const STORAGE_KEY = 'pf:crop-ratio';

export function getCropRatio(): CropRatio {
	if (!browser) return DEFAULT_RATIO;
	const id = localStorage.getItem(STORAGE_KEY);
	return CROP_RATIOS.find((r) => r.id === id) ?? DEFAULT_RATIO;
}

export function setCropRatio(ratio: CropRatio): void {
	if (browser) localStorage.setItem(STORAGE_KEY, ratio.id);
}
