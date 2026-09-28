<script lang="ts">
	import qrcode from 'qrcode-generator';

	let { text, size = 320 }: { text: string; size?: number } = $props();

	// One SVG path of dark modules: cheap to render, crisp at any size.
	const qr = $derived.by(() => {
		const q = qrcode(0, 'M');
		q.addData(text);
		q.make();
		const n = q.getModuleCount();
		let d = '';
		for (let r = 0; r < n; r++) {
			for (let c = 0; c < n; c++) {
				if (q.isDark(r, c)) d += `M${c} ${r}h1v1h-1z`;
			}
		}
		return { n, d };
	});
</script>

<svg
	viewBox="-2 -2 {qr.n + 4} {qr.n + 4}"
	width={size}
	height={size}
	shape-rendering="crispEdges"
	class="rounded-2xl bg-white"
	role="img"
	aria-label={text}
	data-testid="touch-qr"
>
	<path d={qr.d} fill="#000" />
</svg>
