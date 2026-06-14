import { invalidate } from '$app/navigation';
import { apiGetUpdate, apiApplyUpdate, apiCheckUpdate, apiGetLicenses } from '$lib/api/sdk.gen';
import type { UpdateStatusResponse } from '$lib/api/types.gen';
import { toaster } from './toaster';

export type { UpdateStatusResponse };
export type UpdatePhase = UpdateStatusResponse['phase'];

// loadUpdate fetches updater status for the dashboard + settings. Failure is non-fatal:
// the frame may be air-gapped, so callers degrade to "no update info" rather than error.
export async function loadUpdate(
	fetch: typeof globalThis.fetch
): Promise<UpdateStatusResponse | null> {
	try {
		const { data, error } = await apiGetUpdate({ fetch });
		if (error) return null;
		return data ?? null;
	} catch {
		return null;
	}
}

// loadLicenses lazily fetches the third-party notices (plain text) for the About modal.
export async function loadLicenses(fetch: typeof globalThis.fetch): Promise<string | null> {
	try {
		const { data, error } = await apiGetLicenses({ fetch, parseAs: 'text' });
		if (error) return null;
		return typeof data === 'string' ? data : null;
	} catch {
		return null;
	}
}

// A failed apply records why; anything else (e.g. "ok") is success.
function isFailure(result: string): boolean {
	return result.startsWith('failed') || result.startsWith('rolled back');
}

// Phases where an apply is mid-flight (a "checking" run concludes with no result).
export function isApplyInProgress(phase: UpdatePhase): boolean {
	return phase === 'downloading' || phase === 'verifying' || phase === 'applying';
}

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

// checkForUpdate triggers a release-source query, waits for the result (last_check advances),
// refreshes, and surfaces the outcome. Offline is expected, not an error. Returns false only
// on a failed trigger.
export async function checkForUpdate(
	prevLastCheck?: string,
	{ pollMs = 1000, maxPolls = 15 }: { pollMs?: number; maxPolls?: number } = {}
): Promise<boolean> {
	const { error } = await apiCheckUpdate();
	if (error) {
		toaster.error({
			title: 'Could not check for updates',
			description: 'Server returned an error'
		});
		return false;
	}

	for (let i = 0; i < maxPolls; i++) {
		await delay(pollMs);
		const { data } = await apiGetUpdate();
		if (!data?.last_check || data.last_check === prevLastCheck) continue;

		await invalidate('/api/system/update');
		if (!data.last_check_ok) {
			toaster.info({
				title: "Couldn't reach the update server",
				description: 'The frame may be offline. It will keep trying.'
			});
		} else if (data.available) {
			toaster.success({
				title: 'Update available',
				description: `Version ${data.latest} is ready.`
			});
		} else {
			toaster.success({ title: 'Up to date', description: "You're on the latest version." });
		}
		return true;
	}
	await invalidate('/api/system/update'); // outran the budget; the daily check will catch up
	return true;
}

async function isHealthy(): Promise<boolean> {
	try {
		return (await fetch('/healthz')).ok;
	} catch {
		return false; // still restarting
	}
}

// Once the apply swaps the binary the server re-execs into it, so the API drops. Wait for
// /healthz to answer again, then reload onto the new version (mirrors the restart flow). If
// it never comes back within the budget, reload anyway rather than hang on the spinner.
async function reloadOnNewVersion(pollMs: number, maxPolls: number) {
	for (let i = 0; i < maxPolls; i++) {
		await delay(pollMs);
		if (await isHealthy()) break;
	}
	location.reload();
}

// Polls an apply already running to completion (no trigger): a dropped connection means the
// server re-exec'd, so reload onto the new version. prevSeq lets a repeated outcome still
// register as this run's. Used after applyUpdate's trigger, or to adopt a reload mid-update.
export async function followUpdate(
	prevSeq: number,
	onPhase?: (phase: UpdatePhase) => void,
	{ pollMs = 1500, maxPolls = 120 }: { pollMs?: number; maxPolls?: number } = {}
): Promise<boolean> {
	for (let i = 0; i < maxPolls; i++) {
		await delay(pollMs);
		let status: UpdateStatusResponse | undefined;
		try {
			status = (await apiGetUpdate()).data;
		} catch {
			await reloadOnNewVersion(pollMs, maxPolls); // server restarting into the new binary
			return true;
		}
		if (!status) continue;
		onPhase?.(status.phase);

		if (
			status.phase !== 'idle' ||
			!status.last_result ||
			(status.last_result_seq ?? 0) <= prevSeq
		) {
			continue;
		}
		await invalidate('/api/system/update');
		if (isFailure(status.last_result)) {
			toaster.error({ title: 'Update failed', description: status.last_result });
			return false;
		}
		toaster.success({ title: 'Update complete', description: `Now running ${status.current}.` });
		return true;
	}
	await invalidate('/api/system/update');
	return true;
}

// Triggers the apply, then follows it to completion (see followUpdate).
export async function applyUpdate(
	onPhase?: (phase: UpdatePhase) => void,
	opts: { pollMs?: number; maxPolls?: number } = {}
): Promise<boolean> {
	// Baseline the seq before triggering so a repeated outcome still registers as this run's.
	const prevSeq = (await apiGetUpdate()).data?.last_result_seq ?? 0;
	const { error } = await apiApplyUpdate();
	if (error) {
		toaster.error({ title: 'Could not start the update', description: 'Server returned an error' });
		return false;
	}
	return followUpdate(prevSeq, onPhase, opts);
}
