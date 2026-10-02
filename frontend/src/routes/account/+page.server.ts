import { identityFor } from '$lib/server/identity-http';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async (event) => {
	const identity = await identityFor(event).session();
	if (identity.state === 'reconnect_required') {
		event.cookies.delete(event.url.protocol === 'https:' ? '__Host-hub_session' : 'hub_session', {
			path: '/'
		});
	}

	return { identity };
};
