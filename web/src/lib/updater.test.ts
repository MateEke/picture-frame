import { describe, it, expect, vi, afterEach } from 'vitest';
import {
	loadUpdate,
	loadLicenses,
	checkForUpdate,
	applyUpdate,
	followUpdate,
	isApplyInProgress
} from './updater';

const mockGetUpdate = vi.fn();
const mockApplyUpdate = vi.fn();
const mockCheckUpdate = vi.fn();
const mockGetLicenses = vi.fn();
const mockInvalidate = vi.fn();
const mockToasterInfo = vi.fn();
const mockToasterError = vi.fn();
const mockToasterSuccess = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiGetUpdate: (...args: unknown[]) => mockGetUpdate(...args),
	apiApplyUpdate: (...args: unknown[]) => mockApplyUpdate(...args),
	apiCheckUpdate: (...args: unknown[]) => mockCheckUpdate(...args),
	apiGetLicenses: (...args: unknown[]) => mockGetLicenses(...args)
}));

vi.mock('$app/navigation', () => ({
	invalidate: (...args: unknown[]) => mockInvalidate(...args)
}));

vi.mock('./toaster', () => ({
	toaster: {
		info: (...args: unknown[]) => mockToasterInfo(...args),
		error: (...args: unknown[]) => mockToasterError(...args),
		success: (...args: unknown[]) => mockToasterSuccess(...args)
	}
}));

const fast = { pollMs: 1, maxPolls: 5 };

describe('updater', () => {
	afterEach(() => {
		vi.clearAllMocks();
		vi.unstubAllGlobals();
	});

	it('loadUpdate returns parsed status', async () => {
		mockGetUpdate.mockResolvedValue({
			data: { current: 'v1.2.0', available: true },
			error: undefined
		});
		expect(await loadUpdate(fetch)).toEqual({ current: 'v1.2.0', available: true });
		expect(mockGetUpdate).toHaveBeenCalledWith(expect.objectContaining({ fetch }));
	});

	it('loadUpdate degrades to null on error, throw, or missing data', async () => {
		// Error wins even when a body is present: never leak a half-status.
		mockGetUpdate.mockResolvedValue({ data: { current: 'leak' }, error: { detail: 'nope' } });
		expect(await loadUpdate(fetch)).toBeNull();
		mockGetUpdate.mockResolvedValue({ data: undefined, error: undefined });
		expect(await loadUpdate(fetch)).toBeNull();
		mockGetUpdate.mockRejectedValue(new Error('network'));
		expect(await loadUpdate(fetch)).toBeNull();
	});

	it('loadLicenses returns text and passes parseAs:text', async () => {
		mockGetLicenses.mockResolvedValue({ data: 'NOTICES…', error: undefined });
		expect(await loadLicenses(fetch)).toBe('NOTICES…');
		expect(mockGetLicenses).toHaveBeenCalledWith(
			expect.objectContaining({ fetch, parseAs: 'text' })
		);
	});

	it('loadLicenses returns null on error, throw, or non-string data', async () => {
		mockGetLicenses.mockResolvedValue({ data: 'leak', error: { detail: 'x' } });
		expect(await loadLicenses(fetch)).toBeNull();
		mockGetLicenses.mockResolvedValue({ data: { not: 'a string' }, error: undefined });
		expect(await loadLicenses(fetch)).toBeNull();
		mockGetLicenses.mockRejectedValue(new Error('network'));
		expect(await loadLicenses(fetch)).toBeNull();
	});

	it('checkForUpdate toasts and returns false on a failed trigger', async () => {
		mockCheckUpdate.mockResolvedValue({ error: { detail: 'boom' } });
		expect(await checkForUpdate('t0', fast)).toBe(false);
		expect(mockToasterError).toHaveBeenCalledWith({
			title: 'Could not check for updates',
			description: 'Server returned an error'
		});
		expect(mockGetUpdate).not.toHaveBeenCalled();
	});

	it('checkForUpdate keeps polling past empty/unchanged results, then reports availability', async () => {
		mockCheckUpdate.mockResolvedValue({ error: undefined });
		mockGetUpdate
			.mockResolvedValueOnce({ data: undefined }) // no data yet → keep polling
			.mockResolvedValueOnce({ data: { last_check: 't0', last_check_ok: true } }) // unchanged → keep polling
			.mockResolvedValue({
				data: { last_check: 't1', last_check_ok: true, available: true, latest: 'v1.3.1' }
			});
		expect(await checkForUpdate('t0', fast)).toBe(true);
		expect(mockToasterSuccess).toHaveBeenCalledWith({
			title: 'Update available',
			description: 'Version v1.3.1 is ready.'
		});
		expect(mockInvalidate).toHaveBeenCalledWith('/api/system/update');
	});

	it('checkForUpdate reports offline gently when the source is unreachable', async () => {
		mockCheckUpdate.mockResolvedValue({ error: undefined });
		mockGetUpdate.mockResolvedValue({
			data: { last_check: 't1', last_check_ok: false, available: false }
		});
		expect(await checkForUpdate('t0', fast)).toBe(true);
		expect(mockToasterInfo).toHaveBeenCalledWith({
			title: "Couldn't reach the update server",
			description: 'The frame may be offline. It will keep trying.'
		});
		expect(mockToasterError).not.toHaveBeenCalled();
		expect(mockToasterSuccess).not.toHaveBeenCalled();
	});

	it('checkForUpdate reports up-to-date', async () => {
		mockCheckUpdate.mockResolvedValue({ error: undefined });
		mockGetUpdate.mockResolvedValue({
			data: { last_check: 't1', last_check_ok: true, available: false }
		});
		expect(await checkForUpdate('t0', fast)).toBe(true);
		expect(mockToasterSuccess).toHaveBeenCalledWith({
			title: 'Up to date',
			description: "You're on the latest version."
		});
	});

	it('checkForUpdate gives up gracefully when the check never lands', async () => {
		mockCheckUpdate.mockResolvedValue({ error: undefined });
		mockGetUpdate.mockResolvedValue({ data: { last_check: 't0', last_check_ok: true } }); // never advances
		expect(await checkForUpdate('t0', fast)).toBe(true);
		expect(mockGetUpdate).toHaveBeenCalledTimes(fast.maxPolls);
		expect(mockInvalidate).toHaveBeenCalledWith('/api/system/update');
		expect(mockToasterSuccess).not.toHaveBeenCalled();
		expect(mockToasterInfo).not.toHaveBeenCalled();
	});

	it('applyUpdate returns false and toasts on a failed trigger', async () => {
		mockGetUpdate.mockResolvedValue({ data: { last_result: undefined } });
		mockApplyUpdate.mockResolvedValue({ error: { detail: 'boom' } });
		expect(await applyUpdate(undefined, fast)).toBe(false);
		expect(mockToasterError).toHaveBeenCalledWith({
			title: 'Could not start the update',
			description: 'Server returned an error'
		});
	});

	it('applyUpdate reports a pre-swap failure, skipping dropped polls and surfacing phases', async () => {
		mockGetUpdate
			.mockResolvedValueOnce({ data: { last_result: '' } }) // prevSeq baseline before POST
			.mockResolvedValueOnce({ data: undefined }) // dropped poll → keep waiting, no phase
			.mockResolvedValueOnce({ data: { phase: 'verifying', last_result: '' } }) // mid-flight
			.mockResolvedValue({
				data: { phase: 'idle', last_result: 'failed: bad signature', last_result_seq: 1 }
			});
		mockApplyUpdate.mockResolvedValue({ error: undefined });
		const phases: string[] = [];
		expect(await applyUpdate((p) => phases.push(p), fast)).toBe(false);
		expect(phases).toEqual(['verifying', 'idle']);
		expect(mockToasterError).toHaveBeenCalledWith({
			title: 'Update failed',
			description: 'failed: bad signature'
		});
		expect(mockInvalidate).toHaveBeenCalledWith('/api/system/update');
	});

	it('applyUpdate treats a rollback as a failure', async () => {
		mockGetUpdate.mockResolvedValueOnce({ data: { last_result: '' } }).mockResolvedValue({
			data: { phase: 'idle', last_result: 'rolled back from v9.9.9', last_result_seq: 1 }
		});
		mockApplyUpdate.mockResolvedValue({ error: undefined });
		expect(await applyUpdate(undefined, fast)).toBe(false);
		expect(mockToasterError).toHaveBeenCalledWith({
			title: 'Update failed',
			description: 'rolled back from v9.9.9'
		});
	});

	it('applyUpdate terminates on an identical failure with a fresh seq (retry)', async () => {
		// Same outcome string as last time, identical message, higher seq → must terminate, not hang.
		mockGetUpdate
			.mockResolvedValueOnce({ data: { last_result: 'failed: download 404', last_result_seq: 1 } })
			.mockResolvedValue({
				data: { phase: 'idle', last_result: 'failed: download 404', last_result_seq: 2 }
			});
		mockApplyUpdate.mockResolvedValue({ error: undefined });
		expect(await applyUpdate(undefined, fast)).toBe(false);
		expect(mockToasterError).toHaveBeenCalledWith({
			title: 'Update failed',
			description: 'failed: download 404'
		});
	});

	it('applyUpdate reports success when the run finishes', async () => {
		mockGetUpdate
			.mockResolvedValueOnce({ data: undefined }) // no prior result recorded
			.mockResolvedValue({
				data: { phase: 'idle', last_result: 'ok', current: 'v1.3.1', last_result_seq: 1 }
			});
		mockApplyUpdate.mockResolvedValue({ error: undefined });
		expect(await applyUpdate(undefined, fast)).toBe(true);
		expect(mockToasterSuccess).toHaveBeenCalledWith({
			title: 'Update complete',
			description: 'Now running v1.3.1.'
		});
	});

	it('applyUpdate ignores a stale prior result and waits for this run', async () => {
		mockGetUpdate
			.mockResolvedValueOnce({ data: { last_result: 'rolled back from v1', last_result_seq: 1 } }) // stale
			.mockResolvedValueOnce({
				data: { phase: 'idle', last_result: 'rolled back from v1', last_result_seq: 1 }
			}) // same seq → skip
			.mockResolvedValue({
				data: { phase: 'idle', last_result: 'ok', current: 'v2', last_result_seq: 2 }
			});
		mockApplyUpdate.mockResolvedValue({ error: undefined });
		expect(await applyUpdate(undefined, fast)).toBe(true);
		expect(mockToasterSuccess).toHaveBeenCalled();
		expect(mockToasterError).not.toHaveBeenCalled();
	});

	it('applyUpdate stops following after the budget without a verdict', async () => {
		mockGetUpdate
			.mockResolvedValueOnce({ data: { last_result: '' } })
			.mockResolvedValue({ data: { phase: 'downloading', last_result: '' } }); // never terminal
		mockApplyUpdate.mockResolvedValue({ error: undefined });
		expect(await applyUpdate(undefined, fast)).toBe(true);
		expect(mockGetUpdate).toHaveBeenCalledTimes(1 + fast.maxPolls); // prevResult + one per poll
		expect(mockInvalidate).toHaveBeenCalledWith('/api/system/update');
		expect(mockToasterSuccess).not.toHaveBeenCalled();
		expect(mockToasterError).not.toHaveBeenCalled();
	});

	it('applyUpdate reloads onto the new version when the server re-execs mid-apply', async () => {
		const reload = vi.fn();
		vi.stubGlobal('location', { reload });
		// First /healthz probe fails (still restarting), the next succeeds → reload once.
		const fetchStub = vi
			.fn()
			.mockRejectedValueOnce(new Error('down'))
			.mockResolvedValue({ ok: true });
		vi.stubGlobal('fetch', fetchStub);
		mockGetUpdate
			.mockResolvedValueOnce({ data: { last_result: '' } }) // prevResult
			.mockRejectedValue(new Error('connection refused')); // poll → server gone
		mockApplyUpdate.mockResolvedValue({ error: undefined });

		expect(await applyUpdate(undefined, fast)).toBe(true);
		expect(reload).toHaveBeenCalledTimes(1);
		expect(fetchStub).toHaveBeenCalledWith('/healthz');
		expect(fetchStub).toHaveBeenCalledTimes(2);
	});

	it('applyUpdate reloads anyway if the new version never answers within the budget', async () => {
		const reload = vi.fn();
		vi.stubGlobal('location', { reload });
		const fetchStub = vi.fn().mockRejectedValue(new Error('down')); // never healthy
		vi.stubGlobal('fetch', fetchStub);
		mockGetUpdate
			.mockResolvedValueOnce({ data: { last_result: '' } })
			.mockRejectedValue(new Error('connection refused'));
		mockApplyUpdate.mockResolvedValue({ error: undefined });

		expect(await applyUpdate(undefined, fast)).toBe(true);
		expect(reload).toHaveBeenCalledTimes(1);
		expect(fetchStub).toHaveBeenCalledTimes(fast.maxPolls); // probed once per budgeted poll, then gave up
	});

	it('isApplyInProgress is true only for the mid-apply phases', () => {
		expect(isApplyInProgress('downloading')).toBe(true);
		expect(isApplyInProgress('verifying')).toBe(true);
		expect(isApplyInProgress('applying')).toBe(true);
		expect(isApplyInProgress('idle')).toBe(false);
		expect(isApplyInProgress('checking')).toBe(false);
	});

	// followUpdate adopts an apply already running, so it must never POST a fresh apply.
	it('followUpdate adopts an in-flight apply to completion without triggering', async () => {
		mockGetUpdate
			.mockResolvedValueOnce({ data: { phase: 'verifying', last_result: '', last_result_seq: 3 } })
			.mockResolvedValue({
				data: { phase: 'idle', last_result: 'ok', current: 'v2', last_result_seq: 4 }
			});
		const phases: string[] = [];
		expect(await followUpdate(3, (p) => phases.push(p), fast)).toBe(true);
		expect(mockApplyUpdate).not.toHaveBeenCalled();
		expect(phases).toContain('verifying');
		expect(mockToasterSuccess).toHaveBeenCalledWith({
			title: 'Update complete',
			description: 'Now running v2.'
		});
	});

	it('followUpdate reloads onto the new version when the adopted run re-execs', async () => {
		const reload = vi.fn();
		vi.stubGlobal('location', { reload });
		vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true }));
		mockGetUpdate.mockRejectedValue(new Error('connection refused')); // server already gone
		expect(await followUpdate(0, undefined, fast)).toBe(true);
		expect(mockApplyUpdate).not.toHaveBeenCalled();
		expect(reload).toHaveBeenCalledTimes(1);
	});
});
