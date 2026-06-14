import { version } from '$app/environment';

const RELOAD_THROTTLE_MS = 60_000;

// Pure decision for reloadOnBackendVersionChange: reload only on a real version
// mismatch and outside the throttle window. A correct build matches after one
// reload; the throttle (not a permanent block) keeps a mismatched deploy from
// tight-looping yet still recovers a same-version fix.
export function shouldReload(
	backend: string | undefined,
	lastReloadAt: number,
	now: number
): boolean {
	if (!backend || backend === version) return false;
	return now - lastReloadAt >= RELOAD_THROTTLE_MS;
}

// Reloads when the backend reports a build different from this bundle's (a
// self-update swapped the binary). Call once during component init;
// backendVersion should read a reactive source (e.g. SSE).
export function reloadOnBackendVersionChange(backendVersion: () => string | undefined): void {
	$effect(() => {
		const last = Number(sessionStorage.getItem('pf-reload-at') ?? 0);
		if (!shouldReload(backendVersion(), last, Date.now())) return;
		sessionStorage.setItem('pf-reload-at', String(Date.now()));
		location.reload();
	});
}
