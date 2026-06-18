import { version } from '$app/environment';
import { apiHeartbeat } from '$lib/api/sdk.gen';

const HEARTBEAT_INTERVAL_MS = 20_000;

function screenAspect(): number | undefined {
	if (typeof window === 'undefined' || window.innerHeight <= 0) return undefined;
	return window.innerWidth / window.innerHeight;
}

export class Heartbeat {
	private timer: ReturnType<typeof setInterval> | null = null;
	private resizeTimer: ReturnType<typeof setTimeout> | null = null;
	private intervalMs: number;

	constructor(intervalMs: number = HEARTBEAT_INTERVAL_MS) {
		this.intervalMs = intervalMs;
	}

	private async sendBeat(): Promise<void> {
		try {
			await apiHeartbeat({ query: { version, aspect: screenAspect() } });
		} catch {
			// best-effort
		}
	}

	// Report a new aspect promptly on rotation/resize instead of waiting a full interval.
	private readonly onResize = (): void => {
		if (this.resizeTimer !== null) clearTimeout(this.resizeTimer);
		this.resizeTimer = setTimeout(() => this.sendBeat(), 500);
	};

	start(): void {
		if (this.timer !== null) return;
		this.sendBeat();
		this.timer = setInterval(() => this.sendBeat(), this.intervalMs);
		if (typeof window !== 'undefined' && window.addEventListener) {
			window.addEventListener('resize', this.onResize);
		}
	}

	stop(): void {
		if (this.timer !== null) {
			clearInterval(this.timer);
			this.timer = null;
		}
		if (this.resizeTimer !== null) {
			clearTimeout(this.resizeTimer);
			this.resizeTimer = null;
		}
		if (typeof window !== 'undefined' && window.removeEventListener) {
			window.removeEventListener('resize', this.onResize);
		}
	}
}
