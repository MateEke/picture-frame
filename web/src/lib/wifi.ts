import {
	apiGetWifiStatus,
	apiGetWifiNetworks,
	apiWifiConnect,
	apiWifiForget,
	apiConfigureAp
} from '$lib/api/sdk.gen';
import type { WiFiState, WiFiNetwork } from '$lib/api/types.gen';
import { toaster } from './toaster';
export type WiFiMode = 'connected' | 'ethernet' | 'ap' | 'disconnected' | 'connecting';

export async function loadWiFiStatus(
	fetch: typeof globalThis.fetch,
	signal?: AbortSignal
): Promise<WiFiState | null> {
	const { data, error, response } = await apiGetWifiStatus({ fetch, signal });
	if (response?.status === 503) return null;
	if (data) return data;
	throw new Error(`Failed to load WiFi status: ${error ? 'server error' : 'unknown'}`);
}

export async function scanNetworks(): Promise<WiFiNetwork[]> {
	const { data, error } = await apiGetWifiNetworks();
	if (error) throw new Error('Scan failed');
	return data ?? [];
}

export async function connectWiFi(
	ssid: string,
	password: string,
	hidden = false
): Promise<boolean> {
	try {
		const { error, response } = await apiWifiConnect({ body: { ssid, password, hidden } });
		if (!error) return true;
		if (response?.status === 503) {
			toaster.error({ title: 'WiFi busy', description: 'Another connection is in progress.' });
			return false;
		}
		toaster.error({
			title: 'Connect failed',
			description: `Server returned ${response?.status ?? 'unknown'}`
		});
		return false;
	} catch (err) {
		toaster.error({
			title: 'Connect failed',
			description: err instanceof Error ? err.message : 'Unknown error'
		});
		return false;
	}
}

export async function forgetNetwork(ssid: string): Promise<boolean> {
	try {
		const { error } = await apiWifiForget({ path: { ssid } });
		return !error;
	} catch {
		return false;
	}
}

export async function configureAP(
	enabled: boolean,
	ssid: string,
	password?: string
): Promise<WiFiState | null> {
	const body: { enabled: boolean; ssid: string; password?: string } = { enabled, ssid };
	if (password !== undefined) body.password = password;
	try {
		const { data, error } = await apiConfigureAp({ body });
		if (!error) return data;
		toaster.error({ title: 'AP configure failed', description: 'Server returned an error' });
		return null;
	} catch (err) {
		toaster.error({
			title: 'AP configure failed',
			description: err instanceof Error ? err.message : 'Unknown error'
		});
		return null;
	}
}

export function signalLevel(signal: number): number {
	if (signal >= 75) return 4;
	if (signal >= 50) return 3;
	if (signal >= 25) return 2;
	return 1;
}

export function isWPA3Only(security: string): boolean {
	return security.trim() === 'WPA3';
}

export type NetworkTileIcon = 'wifi' | 'wifi-off' | 'ethernet';

export interface NetworkTile {
	label: string;
	value: string;
	sub?: string;
	icon: NetworkTileIcon;
	tone: 'success' | 'surface';
}

export function networkTile(w: WiFiState | null): NetworkTile {
	if (!w) return { label: 'WiFi', value: 'Unavailable', icon: 'wifi-off', tone: 'surface' };
	if (w.mode === 'ethernet') {
		return {
			label: 'Ethernet',
			value: w.ip || 'Connected',
			sub: w.ip ? 'Connected' : undefined,
			icon: 'ethernet',
			tone: 'success'
		};
	}
	if (w.mode === 'ap')
		return {
			label: 'WiFi',
			value: w.ap_ssid || 'Hotspot',
			sub: 'Hotspot mode',
			icon: 'wifi',
			tone: 'success'
		};
	if (w.mode === 'connecting')
		return { label: 'WiFi', value: 'Connecting…', icon: 'wifi', tone: 'success' };
	if (w.mode === 'connected')
		return {
			label: 'WiFi',
			value: w.ssid || 'Connected',
			sub: 'Connected',
			icon: 'wifi',
			tone: 'success'
		};
	return { label: 'WiFi', value: 'Disconnected', icon: 'wifi', tone: 'success' };
}

// Maps the AP password field to configureAP's optional password: a typed value
// is the new key; empty+unset → '' (open); empty+set → undefined (keep stored).
export function apPasswordPayload(typed: string, isSet: boolean): string | undefined {
	if (typed !== '') return typed;
	if (!isSet) return '';
	return undefined;
}

export interface GroupedNetworks {
	saved: WiFiNetwork[];
	available: WiFiNetwork[];
}

// Splits a scan into Saved (known) and Available, each with the active link
// pinned first, then by descending signal.
export function groupNetworks(networks: WiFiNetwork[], activeSSID: string | null): GroupedNetworks {
	const sort = (nets: WiFiNetwork[]): WiFiNetwork[] =>
		[...nets].sort((a, b) => {
			const aActive = a.ssid === activeSSID;
			const bActive = b.ssid === activeSSID;
			if (aActive !== bActive) return aActive ? -1 : 1;
			return b.signal - a.signal;
		});
	return {
		saved: sort(networks.filter((n) => n.known)),
		available: sort(networks.filter((n) => !n.known))
	};
}
