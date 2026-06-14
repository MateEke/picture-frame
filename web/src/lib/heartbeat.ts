import { version } from '$app/environment';
import { apiHeartbeat } from '$lib/api/sdk.gen';

const HEARTBEAT_INTERVAL_MS = 20_000;

export class Heartbeat {
	private timer: ReturnType<typeof setInterval> | null = null;
	private intervalMs: number;

	constructor(intervalMs: number = HEARTBEAT_INTERVAL_MS) {
		this.intervalMs = intervalMs;
	}

	private async sendBeat(): Promise<void> {
		try {
			// This bundle's build; the update commit gate fires only when the new build beats.
			await apiHeartbeat({ query: { version } });
		} catch {
			// Best-effort: retry on the next beat.
		}
	}

	start(): void {
		if (this.timer !== null) return;
		this.sendBeat();
		this.timer = setInterval(() => this.sendBeat(), this.intervalMs);
	}

	stop(): void {
		if (this.timer !== null) {
			clearInterval(this.timer);
			this.timer = null;
		}
	}
}
