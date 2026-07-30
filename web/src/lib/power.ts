import { apiSystemReboot, apiSystemShutdown } from '$lib/api/sdk.gen';
import { toaster } from './toaster';

export type PowerAction = 'reboot' | 'shutdown';

const labels: Record<PowerAction, string> = {
	reboot: 'Reboot failed',
	shutdown: 'Shutdown failed'
};

/** Returns false when the request failed. */
export async function requestPower(action: PowerAction): Promise<boolean> {
	const call = action === 'reboot' ? apiSystemReboot : apiSystemShutdown;
	try {
		const { error } = await call();
		if (error) {
			toaster.error({ title: labels[action], description: 'Server returned an error' });
			return false;
		}
		return true;
	} catch (err) {
		toaster.error({
			title: labels[action],
			description: err instanceof Error ? err.message : 'Unknown error'
		});
		return false;
	}
}
