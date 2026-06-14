import { invalidate } from '$app/navigation';
import { apiGetConfig, apiGetConfigMeta, apiPutConfig, apiSystemRestart } from '$lib/api/sdk.gen';
import type {
	ConfigResponseBody,
	ConfigMetaBody,
	GetConfigData,
	ConfigDtoWritable
} from '$lib/api/types.gen';
import { toaster } from './toaster';

export type { ConfigResponseBody, ConfigMetaBody };

export async function loadConfig(
	fetch: typeof globalThis.fetch
): Promise<ConfigResponseBody | null> {
	try {
		const { data, error } = await apiGetConfig({ fetch });
		if (error) {
			toaster.error({ title: 'Failed to load config', description: 'Server returned an error' });
			return null;
		}
		return data ?? null;
	} catch (err) {
		toaster.error({
			title: 'Failed to load config',
			description: err instanceof Error ? err.message : 'Unknown error'
		});
		return null;
	}
}

export async function loadConfigMeta(
	fetch: typeof globalThis.fetch
): Promise<ConfigMetaBody | null> {
	try {
		const { data, error } = await apiGetConfigMeta({ fetch });
		if (error) {
			toaster.error({
				title: 'Failed to load config meta',
				description: 'Server returned an error'
			});
			return null;
		}
		return data ?? null;
	} catch (err) {
		toaster.error({
			title: 'Failed to load config meta',
			description: err instanceof Error ? err.message : 'Unknown error'
		});
		return null;
	}
}

export type SaveResult = { ok: true; restart_pending: boolean } | { ok: false; detail: string };

export async function saveConfig(cfg: ConfigResponseBody): Promise<SaveResult> {
	const body: ConfigDtoWritable & { restart_pending?: ConfigResponseBody['restart_pending'] } = cfg;
	delete body.restart_pending;

	try {
		const { data, error } = await apiPutConfig({ body });
		if (error) {
			const detail = error.detail ?? `Server returned an error`;
			return { ok: false, detail };
		}
		await invalidate('/api/config' satisfies GetConfigData['url']);
		return { ok: true, restart_pending: data?.restart_pending ?? false };
	} catch (err) {
		const detail = err instanceof Error ? err.message : 'Unknown error';
		toaster.error({ title: 'Save failed', description: detail });
		return { ok: false, detail };
	}
}

export async function restartFrame(): Promise<boolean> {
	try {
		const { error } = await apiSystemRestart();
		if (error) {
			toaster.error({ title: 'Restart failed', description: 'Server returned an error' });
			return false;
		}
		return true;
	} catch (err) {
		toaster.error({
			title: 'Restart failed',
			description: err instanceof Error ? err.message : 'Unknown error'
		});
		return false;
	}
}
