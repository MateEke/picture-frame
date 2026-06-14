import { client } from '$lib/api/client.gen';
import { loginRedirectTarget } from './auth';

// 401 from a gated endpoint → bounce to login. The decision (which 401s to skip and
// how next= is built) lives in loginRedirectTarget so it can be unit-tested; this
// module is just the browser wiring.
const id = client.interceptors.response.use((response: Response) => {
	const target = loginRedirectTarget(response, location.pathname, location.search);
	if (target) location.assign(target);
	return response;
});

// Eject on HMR dispose so dev reloads don't stack duplicate interceptors.
import.meta.hot?.dispose(() => client.interceptors.response.eject(id));
