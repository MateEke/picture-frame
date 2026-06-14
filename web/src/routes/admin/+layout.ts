import { redirect } from '@sveltejs/kit';
import { loadAuthStatus } from '$lib/auth';
import type { LayoutLoad } from './$types';

// Gate every /admin/* route (incl. /admin/wifi, only the captive-portal probes
// stay public); also exposes auth status to the UI.
export const load: LayoutLoad = async ({ fetch, url, depends }) => {
	depends('app:auth');
	const auth = await loadAuthStatus(fetch);
	if (auth.required && !auth.authenticated) {
		redirect(307, `/login?next=${encodeURIComponent(url.pathname + url.search)}`);
	}
	return { auth };
};
