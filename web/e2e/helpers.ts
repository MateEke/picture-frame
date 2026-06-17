import { createServer } from 'node:net';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import os from 'node:os';

const here = path.dirname(fileURLToPath(import.meta.url));

export function repoRoot(): string {
	return path.resolve(here, '..', '..');
}

export function webRoot(): string {
	return path.resolve(here, '..');
}

export function binaryPath(): string {
	return path.join(os.tmpdir(), 'pf-e2e', 'picture-frame');
}

export function seedImagesDir(): string {
	return path.join(here, 'fixtures', 'images');
}

// JPEG pixel size, read from the SOF marker.
export function jpegSize(buf: Buffer): { width: number; height: number } {
	let i = buf.indexOf(Buffer.from([0xff, 0xd8]));
	if (i < 0) throw new Error('no JPEG SOI marker found');
	i += 2;
	while (i + 9 < buf.length) {
		if (buf[i] !== 0xff) {
			i++;
			continue;
		}
		const marker = buf[i + 1];
		// SOFn carries the size; skip DHT/JPG/DAC (C4/C8/CC).
		if (marker >= 0xc0 && marker <= 0xcf && marker !== 0xc4 && marker !== 0xc8 && marker !== 0xcc) {
			return { height: buf.readUInt16BE(i + 5), width: buf.readUInt16BE(i + 7) };
		}
		i += 2 + buf.readUInt16BE(i + 2);
	}
	throw new Error('no JPEG SOF marker found');
}

// Reserve an ephemeral port, then release it for the server to bind.
export function freePort(): Promise<number> {
	return new Promise((resolve, reject) => {
		const srv = createServer();
		srv.on('error', reject);
		srv.listen(0, '127.0.0.1', () => {
			const addr = srv.address();
			if (addr === null || typeof addr === 'string') {
				srv.close();
				reject(new Error('could not determine a free port'));
				return;
			}
			const { port } = addr;
			srv.close(() => resolve(port));
		});
	});
}
