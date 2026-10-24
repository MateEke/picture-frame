<script lang="ts">
	import type { StatsSummary } from '$lib/api/types.gen';

	let { stats }: { stats: StatsSummary | null } = $props();

	function formatHours(hours: number | undefined): string {
		if (hours === undefined || hours >= 900) return '—';
		return `${hours.toFixed(1)} ore`;
	}

	function formatRate(rate: number | undefined): string {
		if (rate === undefined) return '—';
		return `${rate.toFixed(1)}%/ora`;
	}

	function formatObserved(sec: number | undefined): string {
		if (sec === undefined || sec <= 0) return '0 min';
		const mins = Math.floor(sec / 60);
		if (mins < 60) return `${mins} min`;
		const hrs = (mins / 60).toFixed(1);
		return `${hrs} ore`;
	}

	function translateConfidence(confidence: string | undefined): string {
		switch (confidence) {
			case 'high':
				return 'Alta';
			case 'medium':
				return 'Media';
			case 'low':
				return 'Bassa';
			case 'insufficient':
			default:
				return 'Dati insufficienti';
		}
	}
</script>

<div class="battery-analysis space-y-6 rounded-xl border border-neutral-800 bg-neutral-900/60 p-6">
	<div class="flex items-center justify-between border-b border-neutral-800 pb-4">
		<div>
			<h2 class="text-lg font-semibold text-neutral-100">Analisi Autonomia Batteria</h2>
			<p class="text-xs text-neutral-400">
				Confronto scarica Surface 3 su Debian (Schermo Acceso vs Schermo Spento)
			</p>
		</div>
		{#if stats}
			<div class="text-right">
				<span class="text-xs text-neutral-400">Livello attuale:</span>
				<span class="ml-1 text-sm font-semibold tabular-nums text-neutral-200">{stats.current_pct}%</span>
				{#if stats.charging}
					<span class="ml-1 rounded bg-amber-500/20 px-1.5 py-0.5 text-[10px] font-medium text-amber-400">In carica</span>
				{/if}
			</div>
		{/if}
	</div>

	<!-- Side-by-side Cards -->
	<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
		<!-- Screen On Card -->
		<div class="rounded-lg border border-neutral-800 bg-neutral-950/50 p-4">
			<div class="mb-3 flex items-center justify-between">
				<span class="text-sm font-medium text-amber-400 flex items-center gap-1.5">
					<span class="inline-block h-2 w-2 rounded-full bg-amber-400"></span>
					Schermo acceso
				</span>
				<span class="text-xs text-neutral-400">
					Affidabilità: <strong class="font-medium text-neutral-200">{translateConfidence(stats?.screen_on?.confidence)}</strong>
				</span>
			</div>

			{#if !stats || !stats.screen_on?.sufficient_data}
				<div class="py-6 text-center text-sm font-medium text-neutral-500">
					Dati insufficienti
				</div>
			{:else}
				<div class="space-y-2">
					<div class="flex justify-between border-b border-neutral-800/60 py-1.5 text-xs">
						<span class="text-neutral-400">Autonomia (100% → 0%):</span>
						<span class="font-semibold text-neutral-200 tabular-nums">{formatHours(stats.screen_on.estimated_full_hours)}</span>
					</div>
					<div class="flex justify-between border-b border-neutral-800/60 py-1.5 text-xs">
						<span class="text-neutral-400">Autonomia residua ({stats.current_pct}% → 0%):</span>
						<span class="font-semibold text-neutral-200 tabular-nums">{formatHours(stats.screen_on.estimated_rem_hours)}</span>
					</div>
					<div class="flex justify-between border-b border-neutral-800/60 py-1.5 text-xs">
						<span class="text-neutral-400">Scarica media:</span>
						<span class="font-medium text-neutral-200 tabular-nums">{formatRate(stats.screen_on.discharge_rate_pct_hr)}</span>
					</div>
					<div class="flex justify-between py-1.5 text-xs">
						<span class="text-neutral-400">Tempo totale osservato:</span>
						<span class="font-medium text-neutral-200 tabular-nums">{formatObserved(stats.screen_on.observed_duration_sec)}</span>
					</div>
				</div>
			{/if}
		</div>

		<!-- Screen Off Card -->
		<div class="rounded-lg border border-neutral-800 bg-neutral-950/50 p-4">
			<div class="mb-3 flex items-center justify-between">
				<span class="text-sm font-medium text-indigo-400 flex items-center gap-1.5">
					<span class="inline-block h-2 w-2 rounded-full bg-indigo-400"></span>
					Schermo spento
				</span>
				<span class="text-xs text-neutral-400">
					Affidabilità: <strong class="font-medium text-neutral-200">{translateConfidence(stats?.screen_off?.confidence)}</strong>
				</span>
			</div>

			{#if !stats || !stats.screen_off?.sufficient_data}
				<div class="py-6 text-center text-sm font-medium text-neutral-500">
					Dati insufficienti
				</div>
			{:else}
				<div class="space-y-2">
					<div class="flex justify-between border-b border-neutral-800/60 py-1.5 text-xs">
						<span class="text-neutral-400">Autonomia (100% → 0%):</span>
						<span class="font-semibold text-neutral-200 tabular-nums">{formatHours(stats.screen_off.estimated_full_hours)}</span>
					</div>
					<div class="flex justify-between border-b border-neutral-800/60 py-1.5 text-xs">
						<span class="text-neutral-400">Autonomia residua ({stats.current_pct}% → 0%):</span>
						<span class="font-semibold text-neutral-200 tabular-nums">{formatHours(stats.screen_off.estimated_rem_hours)}</span>
					</div>
					<div class="flex justify-between border-b border-neutral-800/60 py-1.5 text-xs">
						<span class="text-neutral-400">Scarica media:</span>
						<span class="font-medium text-neutral-200 tabular-nums">{formatRate(stats.screen_off.discharge_rate_pct_hr)}</span>
					</div>
					<div class="flex justify-between py-1.5 text-xs">
						<span class="text-neutral-400">Tempo totale osservato:</span>
						<span class="font-medium text-neutral-200 tabular-nums">{formatObserved(stats.screen_off.observed_duration_sec)}</span>
					</div>
				</div>
			{/if}
		</div>
	</div>

	<!-- Comparison Summary -->
	{#if stats?.comparison}
		<div class="rounded-md border border-neutral-800/80 bg-neutral-950/30 p-3 text-xs text-neutral-300">
			<span class="font-semibold text-neutral-200">Confronto consumi:</span> {stats.comparison}
		</div>
	{/if}

	<!-- 7-day History Summary -->
	{#if stats?.daily && stats.daily.length > 0}
		<div class="space-y-3 pt-2">
			<h3 class="text-xs font-semibold uppercase tracking-wider text-neutral-400">Storico Ultimi 7 Giorni</h3>
			<div class="grid grid-cols-7 gap-1.5 text-center">
				{#each stats.daily as day}
					<div class="rounded border border-neutral-800/60 bg-neutral-950/40 p-2">
						<div class="text-[10px] text-neutral-500">{day.date.slice(5)}</div>
						<div class="mt-1 text-[11px] font-medium text-amber-400 tabular-nums">
							{day.avg_rate_on_pct_hr ? `${day.avg_rate_on_pct_hr.toFixed(1)}%/h` : '—'}
						</div>
						<div class="text-[10px] text-neutral-400">Acceso</div>
						<div class="mt-1 text-[11px] font-medium text-indigo-400 tabular-nums">
							{day.avg_rate_off_pct_hr ? `${day.avg_rate_off_pct_hr.toFixed(1)}%/h` : '—'}
						</div>
						<div class="text-[10px] text-neutral-400">Spento</div>
					</div>
				{/each}
			</div>
		</div>
	{/if}
</div>
