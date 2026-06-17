import { describe, it, expect } from 'vitest';
import { fitWithin, fileToJpegBlob } from './imageProcessing';

// Browser project: canvas and createImageBitmap are available.
function makeJpeg(w: number, h: number, color = '#c0392b'): Promise<File> {
	const c = document.createElement('canvas');
	c.width = w;
	c.height = h;
	const ctx = c.getContext('2d')!;
	ctx.fillStyle = color;
	ctx.fillRect(0, 0, w, h);
	return new Promise((res) =>
		c.toBlob((b) => res(new File([b!], 'src.jpg', { type: 'image/jpeg' })), 'image/jpeg')
	);
}

async function decode(blob: Blob) {
	const bmp = await createImageBitmap(blob);
	const c = document.createElement('canvas');
	c.width = bmp.width;
	c.height = bmp.height;
	const ctx = c.getContext('2d')!;
	ctx.drawImage(bmp, 0, 0);
	const center = ctx.getImageData(bmp.width >> 1, bmp.height >> 1, 1, 1).data;
	bmp.close();
	return { width: c.width, height: c.height, center };
}

describe('imageProcessing', () => {
	describe('fitWithin', () => {
		it('downscales so the longest edge equals the cap, preserving aspect', () => {
			expect(fitWithin(4000, 3000)).toEqual({ width: 1920, height: 1440 });
			expect(fitWithin(3000, 4000)).toEqual({ width: 1440, height: 1920 });
		});

		it('never upscales a smaller image', () => {
			expect(fitWithin(800, 600)).toEqual({ width: 800, height: 600 });
		});

		it('leaves an image already at the cap unchanged', () => {
			expect(fitWithin(1920, 1080)).toEqual({ width: 1920, height: 1080 });
		});
	});

	describe('fileToJpegBlob', () => {
		it('downscales a large image to the Full HD cap, keeping aspect', async () => {
			const { width, height } = await decode(await fileToJpegBlob(await makeJpeg(3840, 2160)));
			expect(Math.max(width, height)).toBe(1920);
			expect(width / height).toBeCloseTo(16 / 9, 1);
		});

		it('keeps a small image at its original size', async () => {
			const { width, height } = await decode(await fileToJpegBlob(await makeJpeg(120, 90)));
			expect({ width, height }).toEqual({ width: 120, height: 90 });
		});

		it('encodes JPEG and preserves the image content', async () => {
			const blob = await fileToJpegBlob(await makeJpeg(200, 200));
			expect(blob.type).toBe('image/jpeg');
			const { center } = await decode(blob);
			expect(center[0]).toBeGreaterThan(150); // red of #c0392b survives
			expect(center[2]).toBeLessThan(120);
		});
	});
});
