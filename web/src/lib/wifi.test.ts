import { describe, it, expect, vi, afterEach } from 'vitest';
import { toaster } from './toaster';
import {
	loadWiFiStatus,
	scanNetworks,
	connectWiFi,
	forgetNetwork,
	configureAP,
	signalLevel,
	isWPA3Only,
	groupNetworks,
	apPasswordPayload
} from './wifi';
import type { WiFiNetwork } from '$lib/api/types.gen';

vi.mock('./toaster', () => ({
	toaster: { error: vi.fn(), success: vi.fn() }
}));

const mockGetWifiStatus = vi.fn();
const mockGetWifiNetworks = vi.fn();
const mockWifiConnect = vi.fn();
const mockWifiForget = vi.fn();
const mockConfigureAp = vi.fn();

vi.mock('$lib/api/sdk.gen', () => ({
	apiGetWifiStatus: (...args: unknown[]) => mockGetWifiStatus(...args),
	apiGetWifiNetworks: (...args: unknown[]) => mockGetWifiNetworks(...args),
	apiWifiConnect: (...args: unknown[]) => mockWifiConnect(...args),
	apiWifiForget: (...args: unknown[]) => mockWifiForget(...args),
	apiConfigureAp: (...args: unknown[]) => mockConfigureAp(...args)
}));

describe('wifi', () => {
	afterEach(() => {
		vi.clearAllMocks();
	});

	describe('loadWiFiStatus', () => {
		it('returns parsed state on ok response', async () => {
			const state = {
				mode: 'connected',
				ssid: 'HomeNet',
				ip: '192.168.1.5',
				ap_enabled: true,
				hostname: 'frame'
			};
			mockGetWifiStatus.mockResolvedValue({
				data: state,
				error: undefined,
				response: { status: 200 }
			});

			const result = await loadWiFiStatus(fetch);
			expect(result).toMatchObject({ mode: 'connected', ssid: 'HomeNet' });
		});

		it('returns null on 503 (dev mode / wifi not available)', async () => {
			mockGetWifiStatus.mockResolvedValue({
				data: undefined,
				error: { status: 503 },
				response: { status: 503 }
			});

			const result = await loadWiFiStatus(fetch);
			expect(result).toBeNull();
		});

		it('throws on other non-ok responses', async () => {
			mockGetWifiStatus.mockResolvedValue({
				data: undefined,
				error: { status: 500 },
				response: { status: 500 }
			});

			await expect(loadWiFiStatus(fetch)).rejects.toThrow();
		});

		it('passes fetch and signal to the SDK call', async () => {
			const customFetch = vi.fn();
			const signal = new AbortController().signal;
			mockGetWifiStatus.mockResolvedValue({
				data: { mode: 'ap' },
				error: undefined,
				response: { status: 200 }
			});

			await loadWiFiStatus(customFetch, signal);

			expect(mockGetWifiStatus).toHaveBeenCalledWith(
				expect.objectContaining({ fetch: customFetch, signal })
			);
		});

		it('returns data even when the response object is absent', async () => {
			mockGetWifiStatus.mockResolvedValue({
				data: { mode: 'connected' },
				error: undefined,
				response: undefined
			});
			expect(await loadWiFiStatus(fetch)).toMatchObject({ mode: 'connected' });
		});

		it('distinguishes server-error from unknown in the throw message', async () => {
			mockGetWifiStatus.mockResolvedValue({
				data: undefined,
				error: { status: 500 },
				response: { status: 500 }
			});
			await expect(loadWiFiStatus(fetch)).rejects.toThrow(/server error/);

			mockGetWifiStatus.mockResolvedValue({
				data: undefined,
				error: undefined,
				response: { status: 500 }
			});
			await expect(loadWiFiStatus(fetch)).rejects.toThrow(/unknown/);
		});
	});

	describe('scanNetworks', () => {
		it('returns network list on ok response', async () => {
			const nets = [
				{ ssid: 'A', signal: 80, security: 'WPA2', known: true },
				{ ssid: 'B', signal: 40, security: 'WPA3', known: false }
			];
			mockGetWifiNetworks.mockResolvedValue({ data: nets, error: undefined });

			const result = await scanNetworks();
			expect(result).toHaveLength(2);
			expect(result[0].ssid).toBe('A');
		});

		it('throws on failure', async () => {
			mockGetWifiNetworks.mockResolvedValue({ data: undefined, error: { status: 500 } });

			await expect(scanNetworks()).rejects.toThrow('Scan failed');
		});

		it('returns empty array when data is undefined', async () => {
			mockGetWifiNetworks.mockResolvedValue({ data: undefined, error: undefined });

			const result = await scanNetworks();
			expect(result).toEqual([]);
		});
	});

	describe('connectWiFi', () => {
		it('returns true on success', async () => {
			mockWifiConnect.mockResolvedValue({ error: undefined, response: { status: 202 } });

			const ok = await connectWiFi('MyNet', 'pass');
			expect(ok).toBe(true);
		});

		it('sends ssid and password in body', async () => {
			mockWifiConnect.mockResolvedValue({ error: undefined, response: { status: 202 } });

			await connectWiFi('HomeNet', 'secret');

			expect(mockWifiConnect).toHaveBeenCalledWith(
				expect.objectContaining({ body: { ssid: 'HomeNet', password: 'secret' } })
			);
		});

		it('toasts and returns false on 503 busy', async () => {
			mockWifiConnect.mockResolvedValue({ error: { status: 503 }, response: { status: 503 } });

			const ok = await connectWiFi('MyNet', 'pass');
			expect(ok).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith(expect.objectContaining({ title: 'WiFi busy' }));
		});

		it('toasts and returns false on other error status', async () => {
			mockWifiConnect.mockResolvedValue({ error: { status: 500 }, response: { status: 500 } });

			const ok = await connectWiFi('MyNet', 'pass');
			expect(ok).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith(
				expect.objectContaining({ title: 'Connect failed' })
			);
		});

		it('toasts and returns false on fetch rejection', async () => {
			mockWifiConnect.mockRejectedValue(new Error('network down'));

			const ok = await connectWiFi('MyNet', 'pass');
			expect(ok).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith(
				expect.objectContaining({ description: 'network down' })
			);
		});

		it('reports the exact status in the generic failure toast', async () => {
			mockWifiConnect.mockResolvedValue({ error: { status: 500 }, response: { status: 500 } });
			await connectWiFi('MyNet', 'pass');
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Connect failed',
				description: 'Server returned 500'
			});
		});

		it('falls back to "unknown" status when the response is absent', async () => {
			mockWifiConnect.mockResolvedValue({ error: { status: 0 }, response: undefined });
			await connectWiFi('MyNet', 'pass');
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Connect failed',
				description: 'Server returned unknown'
			});
		});

		it('names the busy reason on a 503', async () => {
			mockWifiConnect.mockResolvedValue({ error: { status: 503 }, response: { status: 503 } });
			await connectWiFi('MyNet', 'pass');
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'WiFi busy',
				description: 'Another connection is in progress.'
			});
		});

		it('handles a non-Error rejection', async () => {
			mockWifiConnect.mockRejectedValue('boom');
			expect(await connectWiFi('MyNet', 'pass')).toBe(false);
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'Connect failed',
				description: 'Unknown error'
			});
		});
	});

	describe('forgetNetwork', () => {
		it('returns true on success', async () => {
			mockWifiForget.mockResolvedValue({ error: undefined });

			const ok = await forgetNetwork('OldNet');
			expect(ok).toBe(true);
		});

		it('sends ssid in path param', async () => {
			mockWifiForget.mockResolvedValue({ error: undefined });

			await forgetNetwork('My Net');

			expect(mockWifiForget).toHaveBeenCalledWith(
				expect.objectContaining({ path: { ssid: 'My Net' } })
			);
		});

		it('returns false on error without toasting', async () => {
			mockWifiForget.mockResolvedValue({ error: { status: 500 } });

			const ok = await forgetNetwork('OldNet');
			expect(ok).toBe(false);
			expect(toaster.error).not.toHaveBeenCalled();
		});

		it('returns false when the request rejects', async () => {
			mockWifiForget.mockRejectedValue(new Error('network down'));

			const ok = await forgetNetwork('OldNet');
			expect(ok).toBe(false);
		});
	});

	describe('configureAP', () => {
		it('returns new state on success', async () => {
			const state = { mode: 'connected', ssid: 'PF', ip: '', ap_enabled: true, hostname: 'frame' };
			mockConfigureAp.mockResolvedValue({ data: state, error: undefined });

			const result = await configureAP(true, 'PF', '');
			expect(result).toMatchObject({ ap_enabled: true });
		});

		it('sends enabled and ssid in body', async () => {
			mockConfigureAp.mockResolvedValue({ data: { ap_enabled: true }, error: undefined });

			await configureAP(true, 'PF');

			expect(mockConfigureAp).toHaveBeenCalledWith(
				expect.objectContaining({ body: { enabled: true, ssid: 'PF' } })
			);
		});

		it('includes password in body when provided', async () => {
			mockConfigureAp.mockResolvedValue({ data: { ap_enabled: true }, error: undefined });

			await configureAP(true, 'PF', 'secret');

			expect(mockConfigureAp).toHaveBeenCalledWith(
				expect.objectContaining({ body: { enabled: true, ssid: 'PF', password: 'secret' } })
			);
		});

		it('toasts and returns null on error response', async () => {
			mockConfigureAp.mockResolvedValue({ data: undefined, error: { status: 500 } });

			const result = await configureAP(false, '', '');
			expect(result).toBeNull();
			expect(toaster.error).toHaveBeenCalledWith(
				expect.objectContaining({ title: 'AP configure failed' })
			);
		});

		it('toasts and returns null on fetch rejection', async () => {
			mockConfigureAp.mockRejectedValue(new Error('network down'));

			const result = await configureAP(true, 'PF');
			expect(result).toBeNull();
			expect(toaster.error).toHaveBeenCalledWith(
				expect.objectContaining({ description: 'network down' })
			);
		});

		it('omits the password key entirely when none is provided (keeps stored key)', async () => {
			mockConfigureAp.mockResolvedValue({ data: { ap_enabled: true }, error: undefined });
			await configureAP(true, 'PF');
			expect(mockConfigureAp.mock.calls[0][0].body).not.toHaveProperty('password');
		});

		it('sends an explicit empty password for an open AP', async () => {
			mockConfigureAp.mockResolvedValue({ data: { ap_enabled: true }, error: undefined });
			await configureAP(true, 'PF', '');
			expect(mockConfigureAp.mock.calls[0][0].body).toEqual({
				enabled: true,
				ssid: 'PF',
				password: ''
			});
		});

		it('names the error in the failure toast', async () => {
			mockConfigureAp.mockResolvedValue({ data: undefined, error: { status: 500 } });
			await configureAP(false, '', '');
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'AP configure failed',
				description: 'Server returned an error'
			});
		});

		it('handles a non-Error rejection', async () => {
			mockConfigureAp.mockRejectedValue('boom');
			expect(await configureAP(true, 'PF')).toBeNull();
			expect(toaster.error).toHaveBeenCalledWith({
				title: 'AP configure failed',
				description: 'Unknown error'
			});
		});
	});

	describe('signalLevel', () => {
		it('returns four for strong signal', () => {
			expect(signalLevel(100)).toBe(4);
			expect(signalLevel(75)).toBe(4);
		});

		it('returns three for good signal', () => {
			expect(signalLevel(74)).toBe(3);
			expect(signalLevel(50)).toBe(3);
		});

		it('returns two for medium signal', () => {
			expect(signalLevel(49)).toBe(2);
			expect(signalLevel(25)).toBe(2);
		});

		it('returns one for weak signal (never zero)', () => {
			expect(signalLevel(24)).toBe(1);
			expect(signalLevel(0)).toBe(1);
		});
	});

	describe('isWPA3Only', () => {
		it('returns true for WPA3-only security', () => {
			expect(isWPA3Only('WPA3')).toBe(true);
		});

		it('returns false for mixed WPA2 WPA3', () => {
			expect(isWPA3Only('WPA2 WPA3')).toBe(false);
		});

		it('returns false for WPA2', () => {
			expect(isWPA3Only('WPA2')).toBe(false);
		});

		it('returns false for open network (empty string)', () => {
			expect(isWPA3Only('')).toBe(false);
		});

		it('handles whitespace padding', () => {
			expect(isWPA3Only('  WPA3  ')).toBe(true);
		});
	});

	describe('groupNetworks', () => {
		const net = (ssid: string, signal: number, known: boolean): WiFiNetwork => ({
			ssid,
			signal,
			security: 'WPA2',
			known
		});

		it('splits into saved (known) and available (unknown)', () => {
			const { saved, available } = groupNetworks(
				[net('A', 50, true), net('B', 60, false), net('C', 40, true)],
				null
			);
			expect(saved.map((n) => n.ssid)).toEqual(['A', 'C']);
			expect(available.map((n) => n.ssid)).toEqual(['B']);
		});

		it('sorts each group by descending signal', () => {
			const { available } = groupNetworks(
				[net('low', 20, false), net('high', 90, false), net('mid', 55, false)],
				null
			);
			expect(available.map((n) => n.ssid)).toEqual(['high', 'mid', 'low']);
		});

		it('pins the active network first regardless of signal', () => {
			const { saved } = groupNetworks(
				[net('strong', 95, true), net('active', 30, true), net('weak', 10, true)],
				'active'
			);
			expect(saved.map((n) => n.ssid)).toEqual(['active', 'strong', 'weak']);
		});

		it('returns empty groups for an empty scan', () => {
			expect(groupNetworks([], null)).toEqual({ saved: [], available: [] });
		});
	});

	describe('apPasswordPayload', () => {
		it('sends a typed value as the new key (even when none was set)', () => {
			expect(apPasswordPayload('newpass', false)).toBe('newpass');
		});

		it('sends a typed value as the new key when one was already set', () => {
			expect(apPasswordPayload('newpass', true)).toBe('newpass');
		});

		it('sends "" for an empty field that was cleared or never set (open AP)', () => {
			expect(apPasswordPayload('', false)).toBe('');
		});

		it('keeps the stored key (undefined) for an untouched masked field', () => {
			expect(apPasswordPayload('', true)).toBeUndefined();
		});
	});
});
