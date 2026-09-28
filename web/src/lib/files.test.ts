import { describe, expect, it, vi } from 'vitest';
import { isPhoto, splitUploads, uploadFiles } from './files';

function file(name: string, type: string, size = 10): File {
	return new File([new Uint8Array(size)], name, { type });
}

describe('isPhoto / splitUploads', () => {
	it('routes images to the slideshow and everything else to files', () => {
		expect(isPhoto({ name: 'a.jpg', type: 'image/jpeg' })).toBe(true);
		expect(isPhoto({ name: 'a.heic', type: '' })).toBe(true);
		expect(isPhoto({ name: 'logo.svg', type: 'image/svg+xml' })).toBe(false);
		expect(isPhoto({ name: 'clip.mp4', type: 'video/mp4' })).toBe(false);
		expect(isPhoto({ name: 'notes', type: '' })).toBe(false);

		const { photos, others } = splitUploads([
			{ name: 'a.jpg', type: 'image/jpeg' },
			{ name: 'b.pdf', type: 'application/pdf' },
			{ name: 'c.png', type: 'image/png' }
		]);
		expect(photos.map((f) => f.name)).toEqual(['a.jpg', 'c.png']);
		expect(others.map((f) => f.name)).toEqual(['b.pdf']);
	});
});

describe('uploadFiles', () => {
	it('uploads sequentially and reports byte progress across the batch', async () => {
		const progress: number[] = [];
		const send = vi.fn(async (_f: File, onBytes: (n: number) => void) => {
			onBytes(5);
			return 201;
		});
		const result = await uploadFiles(
			[file('a.mp4', 'video/mp4', 10), file('b.pdf', 'application/pdf', 30)],
			{
				send,
				onProgress: (sent) => progress.push(sent)
			}
		);
		expect(result).toEqual({ added: 2, failed: [], outcome: 'complete' });
		expect(progress).toEqual([5, 10, 15, 40]);
	});

	it('keeps going past a refused file', async () => {
		const statuses = [413, 201];
		const result = await uploadFiles(
			[file('big.mov', 'video/quicktime'), file('ok.txt', 'text/plain')],
			{
				send: async () => statuses.shift() ?? 201
			}
		);
		expect(result).toEqual({ added: 1, failed: ['big.mov'], outcome: 'complete' });
	});

	it('gives up after three unanswered requests in a row', async () => {
		const files = ['a', 'b', 'c', 'd'].map((n) => file(n, 'text/plain'));
		const send = vi.fn(async () => 0);
		const result = await uploadFiles(files, { send });
		expect(result.outcome).toBe('unreachable');
		expect(send).toHaveBeenCalledTimes(3);
	});

	it('stops when aborted', async () => {
		const ctl = new AbortController();
		const send = vi.fn(async () => {
			ctl.abort();
			return -1;
		});
		const result = await uploadFiles([file('a', 'text/plain'), file('b', 'text/plain')], {
			send,
			signal: ctl.signal
		});
		expect(result).toEqual({ added: 0, failed: [], outcome: 'stopped' });
		expect(send).toHaveBeenCalledTimes(1);
	});
});
