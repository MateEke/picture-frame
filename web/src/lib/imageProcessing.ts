const MAX_EDGE = 1920; // long-edge cap (Full HD)
const JPEG_QUALITY = 0.85;

// Downscale-only fit within maxEdge, preserving aspect.
export function fitWithin(
	width: number,
	height: number,
	maxEdge = MAX_EDGE
): { width: number; height: number } {
	const longest = Math.max(width, height);
	if (longest <= maxEdge) return { width, height };
	const scale = maxEdge / longest;
	return { width: Math.round(width * scale), height: Math.round(height * scale) };
}

// No-crop path: downscale to fit, encode JPEG, client-side.
export async function fileToJpegBlob(
	file: File,
	maxEdge = MAX_EDGE,
	quality = JPEG_QUALITY
): Promise<Blob> {
	const bitmap = await createImageBitmap(file);
	try {
		const { width, height } = fitWithin(bitmap.width, bitmap.height, maxEdge);
		const canvas = document.createElement('canvas');
		canvas.width = width;
		canvas.height = height;
		const ctx = canvas.getContext('2d');
		if (!ctx) throw new Error('Could not get a 2D canvas context');
		ctx.drawImage(bitmap, 0, 0, width, height);
		return await new Promise<Blob>((resolve, reject) => {
			canvas.toBlob(
				(blob) => (blob ? resolve(blob) : reject(new Error('Canvas toBlob returned null'))),
				'image/jpeg',
				quality
			);
		});
	} finally {
		bitmap.close();
	}
}
