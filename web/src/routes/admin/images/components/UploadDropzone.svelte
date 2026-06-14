<script lang="ts">
	import { FileUpload } from '@skeletonlabs/skeleton-svelte';
	import { CloudUploadIcon } from '@lucide/svelte';

	let { onFile }: { onFile: (file: File) => void } = $props();

	// One photo at a time: the cropper handles a single file before the next.
	function handleAccept(details: { files: File[] }) {
		const file = details.files[0];
		if (file) onFile(file);
	}
</script>

<FileUpload accept="image/*" maxFiles={1} onFileAccept={handleAccept}>
	<FileUpload.Dropzone
		class="border-surface-300-700 hover:border-primary-500 hover:bg-surface-50-950 cursor-pointer gap-1.5 rounded-lg border-2 border-dashed p-4 text-center transition-colors sm:gap-2 sm:p-8"
	>
		<FileUpload.HiddenInput data-testid="photo-upload-input" />
		<div
			class="bg-primary-500/10 text-primary-500 grid size-9 place-items-center rounded-full sm:size-12"
		>
			<CloudUploadIcon class="size-5 sm:size-6" />
		</div>
		<p class="font-medium">
			<span class="sm:hidden">Add a photo</span>
			<span class="hidden sm:inline">Drop a photo here, or click to choose</span>
		</p>
		<p class="text-surface-500-400 text-sm">You'll crop it to the frame before it's added.</p>
	</FileUpload.Dropzone>
</FileUpload>
