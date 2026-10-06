<script lang="ts">
	import { FileUpload } from '@skeletonlabs/skeleton-svelte';
	import { CloudUploadIcon } from '@lucide/svelte';
	import { toaster } from '$lib/toaster';
	import { rejectedFilesMessage, type RejectedFile } from '../uploadFeedback';

	let { onFiles }: { onFiles: (files: File[]) => void } = $props();

	const maxFiles = 200;

	function handleAccept(details: { files: File[] }) {
		if (details.files.length > 0) onFiles(details.files);
	}

	function handleReject(details: { files: RejectedFile[] }) {
		const { type, ...message } = rejectedFilesMessage(details.files, maxFiles);
		toaster[type](message);
	}
</script>

<FileUpload accept="image/*" {maxFiles} onFileAccept={handleAccept} onFileReject={handleReject}>
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
			<span class="sm:hidden">Add photos</span>
			<span class="hidden sm:inline">Drop photos here, or click to choose</span>
		</p>
		<p class="text-surface-500-400 text-sm">
			One photo opens the cropper. Several are added uncropped.
		</p>
	</FileUpload.Dropzone>
</FileUpload>
