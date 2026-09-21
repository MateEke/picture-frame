import type { BulkUploadResult } from '$lib/images';

// Matches the toaster's method names, so a caller can pick one by type.
export interface ToastMessage {
	type: 'success' | 'info' | 'warning' | 'error';
	title: string;
	description?: string;
}

// Zag's rejection shape, narrowed to the part we read.
export interface RejectedFile {
	errors: string[];
}

const namedFailureLimit = 3;

function photoCount(n: number): string {
	return `${n} ${n === 1 ? 'photo' : 'photos'}`;
}

function failedList(names: string[]): string {
	if (names.length <= namedFailureLimit) return names.join(', ');
	const named = names.slice(0, namedFailureLimit).join(', ');
	return `${named} and ${names.length - namedFailureLimit} more`;
}

export function bulkUploadMessage(result: BulkUploadResult): ToastMessage {
	if (result.outcome === 'unreachable') {
		return {
			type: 'error',
			title: 'The frame stopped accepting photos',
			description: `Added ${photoCount(result.added)} first. Check the frame, then add the rest.`
		};
	}
	if (result.outcome === 'stopped') {
		return { type: 'info', title: `Stopped. Added ${photoCount(result.added)}` };
	}
	if (result.failed.length > 0) {
		return {
			type: 'warning',
			title: `Added ${photoCount(result.added)}`,
			description: `Could not add ${failedList(result.failed)}.`
		};
	}
	return { type: 'success', title: `Added ${photoCount(result.added)}` };
}

export function rejectedFilesMessage(rejected: RejectedFile[], maxFiles: number): ToastMessage {
	// The cap leads: going over it makes the file input reject the whole selection,
	// so nothing at all was added.
	if (rejected.some((file) => file.errors.includes('TOO_MANY_FILES'))) {
		return {
			type: 'warning',
			title: `Add up to ${maxFiles} photos at a time`,
			description: 'Nothing was added. Select fewer photos and try again.'
		};
	}
	return {
		type: 'warning',
		title: 'Some files were left out',
		description: 'Only image files can be added.'
	};
}
