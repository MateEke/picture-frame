// Self-reset window for a stuck crossfade; ample headroom over load + delay + fade.
const DEFAULT_STALL_MS = 30_000;

export class Fader {
	bottomSrc = $state('');
	topSrc = $state('');
	topOp = $state(0.001);
	transitioning = $state(false);

	private loading = false;
	private switchDelayMs: number;
	private stallMs: number;
	private stallTimer: ReturnType<typeof setTimeout> | null = null;

	constructor(switchDelayMs = 1000, stallMs = DEFAULT_STALL_MS) {
		this.switchDelayMs = switchDelayMs;
		this.stallMs = stallMs;
	}

	show(url: string): void {
		if (this.loading) return;
		if (url === this.bottomSrc || url === this.topSrc) return;

		this.loading = true;

		// First image: nothing to fade from.
		if (!this.bottomSrc) {
			this.bottomSrc = url;
			this.loading = false;
			return;
		}

		this.transitioning = true;
		this.topSrc = url;
		// A failed load or lost transitionend would otherwise hold `loading` forever.
		this.armStallTimer();
	}

	onTopLoad(): void {
		if (!this.loading || this.bottomSrc === this.topSrc) return;

		// Top decoded; give the Pi a beat to settle before fading it in. The
		// guard keeps a stale fade-in off an abandoned transition.
		setTimeout(() => {
			if (this.loading) this.topOp = 1;
		}, this.switchDelayMs);
	}

	onTransitionEnd(): void {
		if (this.topOp === 1) {
			// Fade done: promote top to bottom so the next show() starts from it.
			this.bottomSrc = this.topSrc;
		}
	}

	onBottomLoad(): void {
		if (this.loading && this.bottomSrc === this.topSrc) {
			this.transitioning = false;
			this.topOp = 0.001;
			this.loading = false;
			this.clearStallTimer();
		}
	}

	// Failed top load (deleted, 404): abandon the crossfade; the next show() recovers.
	onTopError(): void {
		if (!this.loading) return;
		this.abandonTransition();
	}

	// Mid-promote, like a top error; on a failed first image, clear bottomSrc so
	// a later show() of the same URL can retry.
	onBottomError(): void {
		if (!this.bottomSrc) return;
		if (this.bottomSrc === this.topSrc) {
			this.abandonTransition();
			return;
		}
		this.bottomSrc = '';
		this.loading = false;
	}

	stop(): void {
		this.loading = false;
		this.clearStallTimer();
	}

	// Resets to "showing bottomSrc, idle"; topSrc = bottomSrc (not '') keeps the
	// snapped-back top layer invisible.
	private abandonTransition(): void {
		this.clearStallTimer();
		this.transitioning = false;
		this.topOp = 0.001;
		this.topSrc = this.bottomSrc;
		this.loading = false;
	}

	private armStallTimer(): void {
		this.clearStallTimer();
		this.stallTimer = setTimeout(() => {
			this.stallTimer = null;
			this.abandonTransition();
		}, this.stallMs);
	}

	private clearStallTimer(): void {
		if (this.stallTimer !== null) {
			clearTimeout(this.stallTimer);
			this.stallTimer = null;
		}
	}
}
