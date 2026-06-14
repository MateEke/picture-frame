import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import path from 'node:path';
import { binaryPath, repoRoot } from './helpers';

// Builds the Go binary once. Runs after the webServer's vite build, so
// //go:embed all:build has content to compile.
export default function globalSetup() {
	const bin = binaryPath();
	mkdirSync(path.dirname(bin), { recursive: true });
	execFileSync('go', ['build', '-o', bin, './cmd/picture-frame'], {
		cwd: repoRoot(),
		stdio: 'inherit'
	});
}
