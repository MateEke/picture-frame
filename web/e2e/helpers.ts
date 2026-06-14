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
