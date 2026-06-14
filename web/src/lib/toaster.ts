import { createToaster } from '@skeletonlabs/skeleton-svelte';

// Single shared toaster instance for the admin UI. Imported by the admin
// layout (to mount the Toast.Group) and by pages that need to surface errors.
export const toaster = createToaster({ placement: 'top-end' });
