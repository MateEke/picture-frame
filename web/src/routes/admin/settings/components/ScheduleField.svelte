<script lang="ts">
	import type { ScheduleDto } from '#lib/api/types.gen.js';
	import { XIcon } from '@lucide/svelte';
	import { scheduleSummary } from '#lib/schedule.js';
	import { eq } from '../utils';
	import Field from './Field.svelte';

	let {
		schedule = $bindable(),
		saved,
		locale,
		error
	}: {
		schedule: ScheduleDto;
		saved: ScheduleDto;
		locale: string;
		error?: string;
	} = $props();

	const times: { key: 'off_at' | 'on_at'; id: string; label: string }[] = [
		{ key: 'off_at', id: 'off', label: 'Turn off at' },
		{ key: 'on_at', id: 'on', label: 'Turn on at' }
	];
</script>

<Field
	label="Screen schedule"
	help="Uses the frame's time zone. Works without a motion sensor."
	changed={!eq(schedule, saved)}
	onrevert={() => (schedule = { ...saved })}
	{error}
>
	<div class="grid grid-cols-2 gap-2">
		{#each times as t (t.id)}
			<label class="space-y-1">
				<span class="text-surface-600-400 text-xs">{t.label}</span>
				<div class="relative">
					<input
						class={['input', schedule[t.key] && 'pe-10']}
						type="time"
						bind:value={schedule[t.key]}
						aria-invalid={error ? true : undefined}
						data-testid="schedule-{t.id}"
					/>
					{#if schedule[t.key]}
						<button
							type="button"
							class="btn-icon btn-icon-xs preset-tonal hover:preset-filled-error-500 absolute end-1.5 top-1/2 -translate-y-1/2"
							aria-label="Clear {t.id} time"
							onclick={() => (schedule[t.key] = '')}
							data-testid="schedule-{t.id}-clear"
						>
							<XIcon size={16} />
						</button>
					{/if}
				</div>
			</label>
		{/each}
	</div>
	{#if !error}
		<p class="text-surface-500-400 text-xs" data-testid="schedule-summary">
			{scheduleSummary(schedule.off_at, schedule.on_at, locale)}
		</p>
	{/if}
</Field>
