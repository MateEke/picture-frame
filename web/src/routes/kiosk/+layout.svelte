<script lang="ts">
	import './layout.css';
	import { SSESubscriber, setSSEContext } from '#lib/sse.svelte.js';

	setSSEContext(new SSESubscriber());

	let { children } = $props();
</script>

{@render children()}
