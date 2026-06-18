import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { Heartbeat } from './heartbeat';

const mockHeartbeat = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiHeartbeat: (...args: unknown[]) => mockHeartbeat(...args)
}));

describe('Heartbeat', () => {
	beforeEach(() => {
		mockHeartbeat.mockResolvedValue({ error: undefined });
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.clearAllMocks();
	});

	it('does nothing until start() is called', () => {
		new Heartbeat();
		vi.advanceTimersByTime(60_000);
		expect(mockHeartbeat).not.toHaveBeenCalled();
	});

	it('sends a heartbeat immediately on start(), stamped with the build version', () => {
		const hb = new Heartbeat();
		hb.start();
		expect(mockHeartbeat).toHaveBeenCalledTimes(1);
		expect(mockHeartbeat).toHaveBeenCalledWith(
			expect.objectContaining({ query: expect.objectContaining({ version: expect.any(String) }) })
		);
		hb.stop();
	});

	it('reports the screen aspect ratio (width/height)', () => {
		vi.stubGlobal('window', { innerWidth: 1600, innerHeight: 900 });
		const hb = new Heartbeat();
		hb.start();
		expect(mockHeartbeat).toHaveBeenCalledWith(
			expect.objectContaining({ query: expect.objectContaining({ aspect: 1600 / 900 }) })
		);
		hb.stop();
		vi.unstubAllGlobals();
	});

	it('continues on network errors', async () => {
		mockHeartbeat.mockRejectedValueOnce(new Error('network down'));
		const hb = new Heartbeat();
		hb.start();

		await vi.advanceTimersByTimeAsync(40_000);
		expect(mockHeartbeat).toHaveBeenCalledTimes(3);

		hb.stop();
	});

	it('sends heartbeats every defined interval', () => {
		const hb = new Heartbeat(20_000);
		hb.start();
		expect(mockHeartbeat).toHaveBeenCalledTimes(1);

		vi.advanceTimersByTime(20_000);
		expect(mockHeartbeat).toHaveBeenCalledTimes(2);

		vi.advanceTimersByTime(20_000);
		expect(mockHeartbeat).toHaveBeenCalledTimes(3);

		hb.stop();
	});

	it('stops heartbeats when stopped', () => {
		const hb = new Heartbeat(20_000);
		hb.start();
		expect(mockHeartbeat).toHaveBeenCalledTimes(1);

		hb.stop();
		vi.advanceTimersByTime(60_000);
		expect(mockHeartbeat).toHaveBeenCalledTimes(1);
	});

	it('start() is idempotent', () => {
		const hb = new Heartbeat(20_000);
		hb.start();
		hb.start();
		expect(mockHeartbeat).toHaveBeenCalledTimes(1);

		vi.advanceTimersByTime(20_000);
		expect(mockHeartbeat).toHaveBeenCalledTimes(2);

		hb.stop();
	});
});
