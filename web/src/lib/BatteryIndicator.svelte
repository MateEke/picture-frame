<script lang="ts">
	import type { BatteryPayload } from '$lib/api/types.gen';

	let {
		battery,
		// Base color when the battery is healthy and not charging; the admin page
		// uses the Skeleton surface palette, the kiosk overlay its own fg token.
		fgClass = 'text-surface-200-700',
		// Low/warning/charging tints. Defaults are Skeleton theme colors; the kiosk
		// passes its overlay fg so the indicator reads over a photo instead of a card.
		lowClass = 'text-error-500',
		warnClass = 'text-warning-500',
		chargingClass = 'text-success-500',
		// The kiosk hides the percent (icon only, over a photo); the admin page shows it.
		showPercent = true
	}: {
		battery: BatteryPayload | null;
		fgClass?: string;
		lowClass?: string;
		warnClass?: string;
		chargingClass?: string;
		showPercent?: boolean;
	} = $props();

	// Determine battery icon segments (1-4) based on percentage
	const segments = $derived.by(() => {
		if (!battery) return 0;
		const pct = battery.percent;
		if (pct > 75) return 4;
		if (pct > 50) return 3;
		if (pct > 25) return 2;
		if (pct > 10) return 1;
		return 0;
	});

	const colorClass = $derived.by(() => {
		if (!battery) return fgClass;
		if (battery.charging) return chargingClass;
		if (battery.percent <= 15) return lowClass;
		if (battery.percent <= 30) return warnClass;
		return fgClass;
	});

	const label = $derived.by(() => {
		if (!battery) return '';
		const status = battery.charging ? 'Charging' : battery.status || 'Discharging';
		return `${battery.percent}% — ${status}`;
	});
</script>

{#if battery}
	<div
		class="battery-widget flex items-center gap-1.5 {colorClass}"
		title={label}
		aria-label="Battery: {label}"
	>
		<!-- Battery icon: body + cap -->
		<div class="battery-body relative flex items-center">
			<svg
				width="28"
				height="14"
				viewBox="0 0 28 14"
				fill="none"
				xmlns="http://www.w3.org/2000/svg"
				class="overflow-visible"
			>
				<!-- Battery outline -->
				<rect
					x="0.5"
					y="0.5"
					width="24"
					height="13"
					rx="2.5"
					stroke="currentColor"
					stroke-width="1"
					fill="none"
				/>
				<!-- Cap nub -->
				<rect x="25" y="4" width="3" height="6" rx="1" fill="currentColor" opacity="0.6" />
				<!-- Fill segments -->
				{#if segments >= 1}
					<rect x="2" y="2" width="4.5" height="10" rx="1" fill="currentColor" opacity="0.9" />
				{/if}
				{#if segments >= 2}
					<rect x="7.5" y="2" width="4.5" height="10" rx="1" fill="currentColor" opacity="0.9" />
				{/if}
				{#if segments >= 3}
					<rect x="13" y="2" width="4.5" height="10" rx="1" fill="currentColor" opacity="0.9" />
				{/if}
				{#if segments >= 4}
					<rect x="18.5" y="2" width="4.5" height="10" rx="1" fill="currentColor" opacity="0.9" />
				{/if}
				<!-- Charging bolt -->
				{#if battery.charging}
					<text
						x="12"
						y="11"
						text-anchor="middle"
						font-size="10"
						fill="currentColor"
						font-weight="bold">⚡</text
					>
				{/if}
			</svg>
		</div>
		{#if showPercent}
			<span class="text-xs font-medium tabular-nums">{battery.percent}%</span>
		{/if}
	</div>
{/if}
